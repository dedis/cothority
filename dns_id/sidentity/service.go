/*
Identity is a service that allows storing of key/value pairs that belong to
a given identity that is shared between multiple devices. In order to
add/remove devices or add/remove key/value-pairs, a 'threshold' of devices
need to vote on those changes.

The key/value-pairs are stored in a personal blockchain and signed by the
cothority using forward-links, so that an external observer can check the
collective signatures and be assured that the blockchain is valid.
*/

package sidentity

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	//"runtime/debug"
	"sync"
	"time"

	"github.com/dedis/cothority/dns_id/ca"
	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/cothority/dns_id/swupdate"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "SIdentity"

var IdentityService onet.ServiceID

var dummyVerfier = func(rootAndTimestamp []byte) bool {
	l := len(rootAndTimestamp)
	_, err := bytesToTimestamp(rootAndTimestamp[l-10 : l])
	if err != nil {
		log.Error("Got some invalid timestamp.")
	}
	return true
}

func init() {
	onet.RegisterNewService(ServiceName, newIdentityService)
	IdentityService = onet.ServiceFactory.ServiceID(ServiceName)
	network.RegisterMessage(&StorageMap{})
	network.RegisterMessage(&Storage{})
	network.RegisterMessage(&SyncStart{})
	network.RegisterMessage(&common_structs.SiteInfo{})
	network.RegisterMessage(&common_structs.PushedPublic{})
	network.RegisterMessage(common_structs.MinusOne{})
}

// Service handles identities
type Service struct {
	*onet.ServiceProcessor
	skipchain *skipchain.Client
	ca        *ca.CSRDispatcher
	*StorageMap
	identitiesMutex sync.Mutex
	path            string
	// 'Publics' holds the map between the ServerIdentity of each web server and its public key (to be
	// used by the devices for encryption of the web server's private tls key)
	Publics       map[string]abstract.Point
	EpochDuration time.Duration
	TheRoster     *onet.Roster
	PropagateFunc messaging.PropagationFunc
	SimulFunc     messaging.PropagationFunc
	expected      int
	cnt           int
	cntMut        sync.Mutex
	setupDone     chan bool
	siteInfoList  []*common_structs.SiteInfo
	publicDone    chan bool
	ok            bool
	attachedDone  chan bool
}

// StorageMap holds the map to the storages so it can be marshaled.
type StorageMap struct {
	Identities map[string]*Storage
}

// Storage stores one identity together with the skipblocks.
type Storage struct {
	sync.Mutex
	ID         skipchain.SkipBlockID
	Latest     *common_structs.Config
	Proposed   *common_structs.Config
	Votes      map[string]*crypto.SchnorrSig
	Root       *skipchain.SkipBlock
	Data       *skipchain.SkipBlock
	SkipBlocks map[string]*skipchain.SkipBlock
	CertInfo   *common_structs.CertInfo

	// Latest PoF (on the Latest config)
	PoF *common_structs.SignatureResponse
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl2(s.ServerIdentity(), "Identity received New Protocol event", conf)
	if tn.ProtocolName() == "SIdentityPropagate" {
		return s.ServiceProcessor.NewProtocol(tn, conf)
	} else if tn.ProtocolName() == "Sync" {
		return nil, nil
	}
	log.Lvlf3("%v: Timestamp Service received New Protocol event", s.String())
	pi, err := swupdate.NewCoSiUpdate(tn, dummyVerfier)
	if err != nil {
		log.Lvlf2("%v", err)
	}
	return pi, err
}

/*
 * API messages
 */

// CreateIdentity will register a new SkipChain and add it to our list of
// messagingd identities.
func (s *Service) CreateIdentity(ai *CreateIdentity) (network.Message, onet.ClientError) {
	log.Lvlf2("Request for a new site identity received at server: %v", s.ServerIdentity())
	ids := &Storage{
		Latest: ai.Config,
	}
	log.Lvl2("Creating Root-skipchain")
	var err error
	ids.Root, err = s.skipchain.CreateRoster(ai.Roster, 2, 10,
		skipchain.VerifyNone, nil)
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	log.Lvl2("Creating Data-skipchain")
	ids.Root, ids.Data, err = s.skipchain.CreateData(ids.Root, 2, 10,
		skipchain.VerifyNone, ai.Config)
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	ids.SkipBlocks = make(map[string]*skipchain.SkipBlock)
	ids.setSkipBlock(ids.Data)

	roster := ids.Root.Roster
	ids.ID = ids.Data.Hash
	/*
		// UNCOMMENT IF CAs ARE TO BE USED
		log.Lvlf2("Asking for a cert for site: %v", ids.ID)
		cert, _ := s.ca.SignCert(ai.Config, nil, ids.Data.Hash)
		certinfo := &common_structs.CertInfo{
			Cert:   cert[0],
			SbHash: ids.Data.Hash,
		}
	*/

	// COMMENT IF CAs ARE TO BE USED
	cert := &common_structs.Cert{
		ID:        ids.ID,
		Hash:      []byte{},
		Signature: nil,
		Public:    nil,
	}
	certinfo := &common_structs.CertInfo{
		Cert:   cert,
		SbHash: ids.Data.Hash,
	}

	ids.CertInfo = certinfo

	ids.SkipBlocks = make(map[string]*skipchain.SkipBlock)
	ids.setSkipBlock(ids.Data)
	ids.Votes = make(map[string]*crypto.SchnorrSig)

	replies, err := s.PropagateFunc(roster, &PropagateIdentity{ids}, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}

	s.save()

	log.Lvlf3("------CreateIdentity(): Successfully created a new identity-------")
	return &CreateIdentityReply{
		Root: ids.Root,
		Data: ids.Data,
	}, nil
}

// ConfigUpdate returns a new configuration update
func (s *Service) ConfigUpdate(cu *ConfigUpdate) (network.Message, onet.ClientError) {
	sid := s.getIdentityStorage(cu.ID)
	if sid == nil {
		return nil, onet.NewClientErrorCode(4100, "Didn't find Identity")
	}
	sid.Lock()
	defer sid.Unlock()
	log.Lvl3(s, "Sending config-update")
	return &ConfigUpdateReply{
		Config: sid.Latest,
	}, nil
}

func (s *Storage) setSkipBlock(latest *skipchain.SkipBlock) bool {
	//s.Lock()
	//defer s.Unlock()
	s.SkipBlocks[string(latest.Hash)] = latest
	return true
}

// getSkipBlockByID returns the skip-block or false if it doesn't exist
func (s *Storage) getSkipBlockByID(sbID skipchain.SkipBlockID) (*skipchain.SkipBlock, bool) {
	//s.Lock()
	//defer s.Unlock()
	b, ok := s.SkipBlocks[string(sbID)]
	return b, ok
}

// ProposeSend only stores the proposed configuration internally. Signatures
// come later.
func (s *Service) ProposeSend(p *ProposeSend) (network.Message, onet.ClientError) {
	log.Lvlf2("Storing new proposal")
	sid := s.getIdentityStorage(p.ID)
	if sid == nil {
		log.Lvlf2("Didn't find Identity")
		return nil, onet.NewClientErrorCode(4100, "Didn't find Identity")
	}

	roster := sid.Root.Roster
	replies, err := s.PropagateFunc(roster, p, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil, nil
}

// ProposeUpdate returns an eventual config-proposition
func (s *Service) ProposeUpdate(cnc *ProposeUpdate) (network.Message, onet.ClientError) {
	log.Lvl2(s, "Sending proposal-update to client")
	sid := s.getIdentityStorage(cnc.ID)
	if sid == nil {
		return nil, onet.NewClientErrorCode(4100, "Didn't find Identity")
	}
	sid.Lock()
	defer sid.Unlock()
	return &ProposeUpdateReply{
		Propose: sid.Proposed,
	}, nil
}

// ProposeVote takes int account a vote for the proposed config. It also verifies
// that the voter is in the latest config.
// An empty signature signifies that the vote has been rejected.
func (s *Service) ProposeVote(v *ProposeVote) (network.Message, onet.ClientError) {
	log.Lvl2(s, "Voting on proposal")
	// First verify if the signature is legitimate
	sid := s.getIdentityStorage(v.ID)
	if sid == nil {
		return nil, onet.NewClientErrorCode(4100, "Didn't find identity")
	}

	// Putting this in a function because of the lock which needs to be held
	// over all calls that might return an error.
	err := func() error {
		sid.Lock()
		defer sid.Unlock()
		log.Lvl2("Voting on", sid.Proposed.Device)
		owner, ok := sid.Latest.Device[v.Signer]
		if !ok {
			log.Lvlf2("Didn't find signer: %v", v.Signer)
			return errors.New("Didn't find signer")
		}
		if sid.Proposed == nil {
			log.Lvlf2("No proposed block")
			return errors.New("No proposed block")
		}
		hash, err := sid.Proposed.Hash()
		if err != nil {
			log.Lvlf2("Couldn't get hash")
			return errors.New("Couldn't get hash")
		}
		if _, exists := sid.Votes[v.Signer]; exists {
			log.Lvlf2("Already voted for that block")
			return errors.New("Already voted for that block")
		}

		// Check whether our clock is relatively close or not to the proposed timestamp
		err2 := sid.Proposed.CheckTimeDiff(maxdiff_sign)
		if err2 != nil {
			log.Lvlf2("Cothority %v", err2)
			return err2
		}

		log.Lvl3(v.Signer, "voted", v.Signature)
		if v.Signature != nil {
			err = crypto.VerifySchnorr(network.Suite, owner.Point, hash, *v.Signature)
			if err != nil {
				log.Lvlf2("%v", err)
				return errors.New("Wrong signature: " + err.Error())
			}
		}
		return nil
	}()
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	// Propagate the vote
	_, err = s.PropagateFunc(sid.Root.Roster, v, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	storage := sid.Copy()

	if len(storage.Votes) < storage.Latest.Threshold && len(storage.Votes) != len(storage.Latest.Device) {
		return nil, nil
	}

	// If we have enough signatures, make a new data-skipblock and
	// propagate it
	log.Lvl3("Having majority or all votes")

	// Making a new data-skipblock
	log.Lvl3("Sending data-block with", storage.Proposed.Device)
	reply, err := s.skipchain.ProposeData(storage.Root, storage.Data, storage.Proposed)
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	skipblock_previous := reply.Previous
	skipblock_latest := reply.Latest
	_, msgLatest, err := network.Unmarshal(skipblock_latest.Data)
	if err != nil {
		log.Error(err)
		return nil, onet.NewClientError(err)
	}
	al, ok := msgLatest.(*common_structs.Config)
	if !ok {
		log.Error(err)
		return nil, onet.NewClientError(err)
	}
	storage.Data = skipblock_latest
	storage.Latest = al
	storage.Proposed = nil
	storage.setSkipBlock(skipblock_latest)
	storage.setSkipBlock(skipblock_previous)
	storage.Votes = make(map[string]*crypto.SchnorrSig)

	usb := &UpdateSkipBlock{
		ID:         v.ID,
		Storage:    storage,
		SbPrevious: skipblock_previous,
	}

	roster := storage.Root.Roster
	/*
		replies, err2 := messaging.PropagateStartAndWaitf(s.Context, roster,
			&LockIdentities{}, propagateTimeout, s.Propagate)
		if err2 != nil {
			return false, err2
		}
		if replies != len(roster.List) {
			log.Warn("Did only get", replies, "out of", len(roster.List))
		}
	*/
	_, err = s.PropagateFunc(roster, usb, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	s.save()
	return &ProposeVoteReply{storage.Data}, nil

}

/*
 * Internal messages
 */

// Propagate handles propagation of all data in the identity-service
func (s *Service) Propagate(msg network.Message) {
	log.Lvlf4("Got msg %+v %v", msg, reflect.TypeOf(msg).String())
	id := skipchain.SkipBlockID(nil)
	var sid *Storage
	switch msg.(type) {
	case *LockIdentities:
		s.identitiesMutex.Lock()
		return
	case *PushPublicKey:
		//log.Print("public key to be stored at server", s.ServerIdentity())
		p := msg.(*PushPublicKey)
		public := p.Public
		serverID := p.ServerID
		key := fmt.Sprintf("tls:%v", serverID)
		s.identitiesMutex.Lock()
		defer s.identitiesMutex.Unlock()
		s.Publics[key] = public
		return
	case *ProposeSend:
		id = msg.(*ProposeSend).ID
	case *ProposeVote:
		id = msg.(*ProposeVote).ID
	case *UpdateSkipBlock:
		id = msg.(*UpdateSkipBlock).ID
		storage := msg.(*UpdateSkipBlock).Storage
		sbprevious := msg.(*UpdateSkipBlock).SbPrevious
		sid = s.getIdentityStorage(id)
		sid.Lock()
		defer sid.Unlock()
		sid.Data = storage.Data
		sid.Latest = storage.Latest
		sid.Proposed = nil
		sid.setSkipBlock(sid.Data)
		sid.setSkipBlock(sbprevious)
		sid.Votes = make(map[string]*crypto.SchnorrSig)
		log.Lvlf2("Skipblock with hash: %v has been stored at server: %v", sid.Data.Hash, s.ServerIdentity())
		return
	case *PropagateIdentity:
		log.Lvlf2("Storing new site identity..")
		pi := msg.(*PropagateIdentity)
		id = pi.Data.Hash
		if s.getIdentityStorage(id) != nil {
			log.Error("Couldn't store new identity")
			return
		}
		sid = pi.Storage
		sid.Votes = make(map[string]*crypto.SchnorrSig)
		s.setIdentityStorage(id, sid)
		return
	case *PropagateCert:
		pc := msg.(*PropagateCert)
		cert := pc.CertInfo.Cert
		id = cert.ID
		sid = pc.Storage
		s.setIdentityStorage(id, sid)
		log.Lvlf3("Fresh cert is now stored")
		return
	case *PropagatePoF:
		log.Lvlf2("Trying to store PoFs at: %v", s.String())
		sids := msg.(*PropagatePoF).Storages
		var identifier int
		for _, storage := range sids {
			id = storage.ID
			sid = s.getIdentityStorage(id)
			sid.Lock()
			defer sid.Unlock()
			sid.PoF = storage.PoF
			identifier = storage.PoF.Identifier
		}
		log.Lvlf2("PoFs (identifier: %v) are now stored at: %v", identifier, s.String())
		return
	}

	if id != nil {
		sid = s.getIdentityStorage(id)
		if sid == nil {
			log.Error("Didn't find entity in", s)
			return
		}
		sid.Lock()
		switch msg.(type) {
		case *ProposeSend:
			log.Lvlf2("Storing proposal..")
			p := msg.(*ProposeSend)
			sid.Proposed = p.Config
			log.Lvlf3("num of votes: %v", len(sid.Votes))
		case *ProposeVote:
			v := msg.(*ProposeVote)
			log.Lvlf2("Storing vote of signer: %v on proposal..", v.Signer)
			log.Lvlf2("num of votes (without counting our vote): %v", len(sid.Votes))
			if len(sid.Votes) == 0 {
				sid.Votes = make(map[string]*crypto.SchnorrSig)
			}
			sid.Votes[v.Signer] = v.Signature
			sid.Proposed.Device[v.Signer].Vote = v.Signature
		}
		sid.Unlock()
	}
}

// Forward traversal of the skipchain from the oldest block as the latter is
// specified by its hash in the request's 'Hash1' field (if Hash1==[]byte{0}, then start
// fetching from the skipblock for the config of which the latest cert is acquired) until
// finding the newest block as it is specified by its hash in the request's 'Hash2' field
// (if Hash2==[]byte{0}, then fetch all skipblocks until the current skipchain head one).
// Skipblocks will be returned from the oldest to the newest
func (s *Service) GetValidSbPath(req *GetValidSbPath) (network.Message, onet.ClientError) {
	id := req.ID
	h1 := req.Hash1
	h2 := req.Hash2
	sid := s.getIdentityStorage(id)
	if sid == nil {
		log.Lvlf2("Didn't find identity: %v", id)
		return nil, onet.NewClientErrorCode(4100, "Didn't find identity")
	}
	log.Lvlf2("server: %v, site: %v - GetValidSbPath(): Start", s.String(), sid.ID)

	sid.Lock()
	defer sid.Unlock()

	var ok bool
	var sb1 *skipchain.SkipBlock
	if !bytes.Equal(h1, []byte{0}) {
		sb1, ok = sid.getSkipBlockByID(h1)
		if !ok {
			log.Lvlf2("server: %v, site: %v - NO VALID PATH: Skipblock with hash: %v not found", s.String(), sid.ID, h1)
			return nil, onet.NewClientErrorCode(4100, "NO VALID PATH")
		}
	} else {
		// fetch all the blocks starting from the one for the config of
		// which the latest cert is acquired
		/*
			_, err = s.CheckRefreshCert(id)
			if err != nil {
				return nil, err
			}
		*/
		h1 = sid.CertInfo.SbHash
		sb1, ok = sid.getSkipBlockByID(h1)
		if !ok {
			log.Lvlf2("NO VALID PATH: Skipblock with hash: %v not found", h1)
			return nil, onet.NewClientErrorCode(4100, "NO VALID PATH")
		}
		log.Lvlf3("Last certified skipblock has hash: %v", h1)
	}

	var sb2 *skipchain.SkipBlock
	if !bytes.Equal(h2, []byte{0}) {
		sb2, ok = sid.getSkipBlockByID(h2)
		if !ok {
			log.Lvlf2("NO VALID PATH: Skipblock with hash: %v not found", h2)
			return nil, onet.NewClientErrorCode(4100, "NO VALID PATH")
		}
	} else {
		// fetch skipblocks until finding the current head of the skipchain
		h2 = sid.Data.Hash
		sb2 = sid.Data
		log.Lvlf2("Current head skipblock has hash: %v", h2)
	}

	oldest := sb1
	newest := sb2

	log.Lvlf3("Oldest skipblock has hash: %v", oldest.Hash)
	log.Lvlf3("Newest skipblock has hash: %v", newest.Hash)
	sbs := make([]*skipchain.SkipBlock, 0)
	sbs = append(sbs, oldest)
	block := oldest
	log.Lvlf3("Appending skipblock with hash: %v", block.Hash)
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[0]
		hash := link.Hash
		log.Lvlf3("Appending skipblock with hash: %v", hash)
		block, ok = sid.getSkipBlockByID(hash)
		if !ok {
			log.Lvlf3("Skipblock with hash: %v not found", hash)
			return nil, onet.NewClientErrorCode(4100, "Skipblock not found")
		}
		sbs = append(sbs, block)
		if bytes.Equal(hash, sid.Data.Hash) || bytes.Equal(hash, newest.Hash) {
			break
		}
	}

	log.Lvlf3("GetValidSbPath(): End with num of returned blocks: %v", len(sbs))
	return &GetValidSbPathReply{Skipblocks: sbs,
		Cert: sid.CertInfo.Cert,
		Hash: sid.CertInfo.SbHash,
		PoF:  sid.PoF,
	}, nil
}

func (s *Service) GetCert(req *GetCert) (network.Message, onet.ClientError) {
	sid := s.getIdentityStorage(req.ID)
	if sid == nil {
		log.Lvlf2("Didn't find identity")
		return nil, onet.NewClientErrorCode(4100, "Didn't find identity")
	}
	sid.Lock()
	defer sid.Unlock()
	/*
		_, err := s.CheckRefreshCert(req.ID)
		if err != nil {
			return nil, err
		}
	*/
	cert := sid.CertInfo.Cert
	hash := sid.CertInfo.SbHash
	return &GetCertReply{Cert: cert, SbHash: hash}, nil
}

func (s *Service) GetPoF(req *GetPoF) (network.Message, onet.ClientError) {
	sid := s.getIdentityStorage(req.ID)
	if sid == nil {
		log.Lvlf2("Didn't find identity")
		return nil, onet.NewClientErrorCode(4100, "Didn't find identity")
	}

	sid.Lock()
	defer sid.Unlock()

	pof := sid.PoF
	hash := sid.Data.Hash
	return &GetPoFReply{PoF: pof, SbHash: hash}, nil
}

// Checks whether the current valid cert for a given site is going to expire soon/it has already expired
// in which case a fresh cert by a CA should be acquired
func (s *Service) CheckRefreshCert(id skipchain.SkipBlockID) (bool, error) {
	sid := s.getIdentityStorage(id)
	if sid == nil {
		log.Lvlf2("Didn't find identity")
		return false, errors.New("Didn't find identity")
	}

	cert_sb_ID := sid.CertInfo.SbHash // hash of the skipblock whose config is the latest certified one
	cert_sb, _ := sid.getSkipBlockByID(cert_sb_ID)
	_, data, _ := network.Unmarshal(cert_sb.Data)
	cert_conf, _ := data.(*common_structs.Config)
	diff := time.Since(time.Unix(0, cert_conf.Timestamp*1000000))
	diff_int := diff.Nanoseconds() / 1000000

	if cert_conf.MaxDuration-diff_int >= refresh_bound {
		log.Lvlf2("We will not get a fresh cert today because the old one is still \"very\" valid")
		return false, nil
	}

	// Get a fresh cert for the 'latestconf' which is included into the site skipchain's current head block
	_, data, _ = network.Unmarshal(sid.Data.Data)
	latestconf, _ := data.(*common_structs.Config)

	var prevconf *common_structs.Config
	if !bytes.Equal(id, sid.Data.Hash) { // if site's skipchain is constituted of more than one (the genesis) skiblocks
		// Find 'prevconf' which is included into the second latest head skipblock of the skipchain
		prevhash := sid.Data.BackLinkIds[0]
		prevblock, ok := sid.getSkipBlockByID(prevhash)
		if !ok {
			log.Lvlf2("Skipblock with hash: %v not found", prevhash)
			return false, fmt.Errorf("Skipblock with hash: %v not found", prevhash)
		}
		_, data, _ = network.Unmarshal(prevblock.Data)
		prevconf, _ = data.(*common_structs.Config)
	} else {
		prevconf = nil
	}

	// Ask for a cert for the 'latestconf'
	log.Lvlf2("Asking for a cert for site: %v", sid.ID)
	cert, _ := s.ca.SignCert(latestconf, prevconf, id)
	log.Lvlf3("[site: %v] num of certs: %v", sid.ID, len(cert))
	certinfo := &common_structs.CertInfo{
		Cert:   cert[0],
		SbHash: sid.Data.Hash,
	}
	sid.CertInfo = certinfo

	roster := sid.Root.Roster
	replies, err := s.PropagateFunc(roster, &PropagateCert{sid}, propagateTimeout)
	if err != nil {
		return false, err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}

	log.Lvlf2("_ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ ")
	log.Lvlf2("CERT REFRESHED!")
	log.Lvlf2("_ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ ")
	return true, nil
}

func (s *Service) PushPublicKey(req *PushPublicKey) (network.Message, onet.ClientError) {
	log.LLvlf2("Public key of a ws received at server: %v", s.ServerIdentity())
	roster := req.Roster
	//public := req.Public
	//serverID := req.ServerID

	//key := fmt.Sprintf("tls:%v", serverID)
	//s.Publics[key] = public

	replies, err := s.PropagateFunc(roster, req, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}

	return &PushPublicKeyReply{}, nil
}

func (s *Service) PullPublicKey(req *PullPublicKey) (network.Message, onet.ClientError) {
	log.Lvlf3("PullPublicKey(): Start")

	serverID := req.ServerID

	key := fmt.Sprintf("tls:%v", serverID)
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	public := s.Publics[key]

	return &PullPublicKeyReply{Public: public}, nil
}

// getIdentityStorage returns the corresponding IdentityStorage or nil
// if none was found
func (s *Service) getIdentityStorage(id skipchain.SkipBlockID) *Storage {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	is, ok := s.Identities[string(id)]
	if !ok {
		return nil
	}
	return is
}

// setIdentityStorage saves an IdentityStorage
func (s *Service) setIdentityStorage(id skipchain.SkipBlockID, is *Storage) {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	log.Lvlf3("%s %x %v", s.Context.ServerIdentity(), id[0:8], is.Latest.Device)
	s.Identities[string(id)] = is
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	return
	b, err := network.Marshal(s.StorageMap)
	if err != nil {
		log.Error("Couldn't marshal service:", err)
	} else {
		err = ioutil.WriteFile(s.path+"/sidentity.bin", b, 0660)
		if err != nil {
			log.Error("Couldn't save file:", err)
		}
	}
}

func (s *Service) ClearIdentities() {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	s.Identities = make(map[string]*Storage)
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	configFile := s.path + "/sidentity.bin"
	b, err := ioutil.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Error while reading %s: %s", configFile, err)
	}
	if len(b) > 0 {
		_, msg, err := network.Unmarshal(b)
		if err != nil {
			return fmt.Errorf("!!! Couldn't unmarshal: %s", err)
		}
		log.Lvl3("Successfully loaded")
		s.StorageMap = msg.(*StorageMap)
	}
	return nil
}

func (s *Service) RunLoop(roster *onet.Roster) {
	c := time.Tick(s.EpochDuration)
	log.Lvlf2("_______________________________________________________")
	log.Lvlf2("------------------TIMESTAMPER BEGINS-------------------")
	log.Lvlf2("_______________________________________________________")
	cnt := 0

	for now := range c {
		cnt++
		log.Lvlf3("_______________________________________________________")
		log.Lvlf3("START OF A TIMESTAMPER ROUND")
		log.Lvlf3("_______________________________________________________")
		data := make([][]byte, 0)
		data2 := make([]common_structs.HashID, 0)
		ids := make([]skipchain.SkipBlockID, 0)

		s.identitiesMutex.Lock()
		identities := s.Identities
		s.identitiesMutex.Unlock()

		for _, sid := range identities {
			latestconf := sid.Latest
			hash, _ := latestconf.Hash()
			data = append(data, []byte(hash))
			data2 = append(data2, hash)
			log.Lvlf3("site: %v, %v", latestconf.FQDN, []byte(hash))
			ids = append(ids, sid.ID)
		}
		num := len(identities)

		if num > 0 {
			log.Lvl2("------- Signing tree root with timestamp:", now, "got", num, "requests")

			// create merkle tree and message to be signed:
			root, proofs := common_structs.ProofTree(sha256.New, data2)
			timestamp := time.Now().Unix() * 1000
			msg := RecreateSignedMsg(root, timestamp)
			log.Lvlf3("------ Before signing")
			for _, server := range roster.List {
				log.Lvlf3("%v", server)
			}

			signature := s.cosiSign(roster, msg)

			log.Lvlf2("--------- %s: Signed a message.\n", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))

			i := 0
			//log.Lvlf3("sites: %v proofs: %v", len(s.Identities), len(proofs))
			//log.Lvlf3("root hash: %v", []byte(root))
			//log.Lvlf3("timestamp: %v", timestamp)
			//log.Lvlf3("signature: %v", signature)
			sids := make([]*Storage, 0)

			for _, id := range ids {
				sid := identities[string(id)]
				pof := &common_structs.SignatureResponse{
					ID:        id,
					Timestamp: timestamp,
					Proof:     proofs[i],
					Root:      root,
					// Collective signature on Timestamp||hash(treeroot)
					Signature:  signature,
					Identifier: cnt,
				}

				// check the validity of pofs
				signedmsg := RecreateSignedMsg(root, timestamp)
				publics := make([]abstract.Point, 0)
				for _, proxy := range roster.List {
					publics = append(publics, proxy.Public)
				}
				err := swupdate.VerifySignature(network.Suite, publics, signedmsg, signature)
				if err != nil {
					log.Lvlf2("Warm Key Holders' signature doesn't verify")
				}
				// verify inclusion proof
				origmsg := data2[i]
				log.Lvlf3("site: %v", sid.Latest.FQDN)
				log.Lvlf3("%v", []byte(origmsg))
				validproof := pof.Proof.Check(sha256.New, root, []byte(origmsg))
				if !validproof {
					log.Lvlf2("Invalid inclusion proof!")
				}
				sid.PoF = pof
				sids = append(sids, sid)
				i++
			}

			log.Lvlf3("Everything OK with the proofs")
			replies, err := s.PropagateFunc(roster, &PropagatePoF{Storages: sids}, propagateTimeout)

			if err != nil {
				log.ErrFatal(err, "Couldn't send")
			}

			if replies != len(roster.List) {
				log.Warn("Did only get", replies, "out of", len(roster.List))
			}

		} else {
			log.Lvl3("No follow-sites at epoch:", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
		}
		log.Lvlf3("_______________________________________________________")
		log.Lvlf3("END OF A TIMESTAMPER ROUND")
		log.Lvlf3("_______________________________________________________")
		//debug.FreeOSMemory()
	}
}

//func (s *Service) cosiSign(roster *onet.Roster, msg []byte) []byte {
func (s *Service) cosiSign(roster *onet.Roster, msg []byte) []byte {
	log.Lvlf2("server: %s", s.String())
	sdaTree := roster.GenerateBinaryTree()
	tni := s.NewTreeNodeInstance(sdaTree, sdaTree.Root, swupdate.ProtocolName)
	pi, err := swupdate.NewCoSiUpdate(tni, dummyVerfier)
	if err != nil {
		log.Lvl2("Couldn't make new protocol: " + err.Error())
		panic("Couldn't make new protocol: " + err.Error())
	}
	s.RegisterProtocolInstance(pi)

	pi.SigningMessage(msg)
	// Take the raw message (already expecting a hash for the timestamp
	// service)
	response := make(chan []byte)
	pi.RegisterSignatureHook(func(sig []byte) {
		response <- sig
	})

	go pi.Dispatch()

	go pi.Start()

	res := <-response
	log.Print("cosiSign(): Received cosi response")
	return res

}

// RecreateSignedMsg is a helper that can be used by the client to recreate the
// message signed by the timestamp service (which is treeroot||timestamp)
func RecreateSignedMsg(treeroot []byte, timestamp int64) []byte {
	timeB := timestampToBytes(timestamp)
	m := make([]byte, len(treeroot)+len(timeB))
	m = append(m, treeroot...)
	m = append(m, timeB...)
	return m
}

func newIdentityService(c *onet.Context) onet.Service {
	log.Print(c)
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		skipchain:        skipchain.NewClient(),
		ca:               ca.NewCSRDispatcher(),
		StorageMap:       &StorageMap{Identities: make(map[string]*Storage)},
		Publics:          make(map[string]abstract.Point),
		//EpochDuration:    time.Millisecond * 250000,
		EpochDuration: time.Millisecond * 1000 * 10,
		setupDone:     make(chan bool),
		publicDone:    make(chan bool),
		attachedDone:  make(chan bool),
	}
	s.ClearIdentities()
	//if err := s.tryLoad(); err != nil {
	//	log.Error(err)
	//}
	var err error
	s.PropagateFunc, err = messaging.NewPropagationFunc(c, "SIdentityPropagate", s.Propagate)
	log.ErrFatal(err)
	s.SimulFunc, err = messaging.NewPropagationFunc(c, "Sync", s.StartSimul)
	log.ErrFatal(err)
	//ws = c.Service("webserver")
	for _, f := range []interface{}{s.ProposeSend, s.ProposeVote,
		s.CreateIdentity, s.ProposeUpdate, s.ConfigUpdate,
		s.GetValidSbPath, s.PushPublicKey, s.PullPublicKey, s.GetCert, s.GetPoF, s.LetsStart, s.LetsEvolve, s.PushedPublic,
	} {
		if err := s.RegisterHandler(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return s
}

//
type SyncStart struct {
	Roster     *onet.Roster
	Clients    int
	Webservers int
	Cothority  int
	Evol1      int
	Evol2      int
}

func (s *Service) StartSimul(msg network.Message) {
	log.Print("StartSimul", s.Context.String())
	m := msg.(*SyncStart)

	// [ clients, webservers, cold key holders, cothority]
	index_client := 0
	index_ws := index_client + m.Clients
	index_CK := index_ws + m.Webservers
	index_WK := index_CK + m.Webservers

	index, _ := m.Roster.Search(s.Context.ServerIdentity().ID)

	roster_WK := onet.NewRoster(m.Roster.List[index_WK:])
	log.Print(s.Context, " INDEX ", index, " VS", index_ws, " ", index_CK, " ", index_WK)

	switch {
	case index < index_ws:
		// client case
		return
	case index < index_CK:
		// webserver case
		go startWs(m.Roster, roster_WK, index_CK+(index-index_ws), index)
		return
	case index < index_WK:
		// cold key case
		go func() {
			<-s.publicDone
			/*for ;; {
				if s.ok==true {
					break
				}
			}*/
			log.Print(s.Context, "ColdKeyHolder is now ready to pursue")
			randWkh := index_WK + rand.Int()%(len(m.Roster.List)-index_WK)
			s.startCkh(m.Roster, roster_WK, randWkh, index_ws+(index-index_CK), m.Evol1, m.Evol2)
		}()
	default:
		// cothority warm key case
		if index == index_WK {
			go s.RunLoop(roster_WK)
		}
	}

}

func (s *Service) startCkh(roster, roster_WK *onet.Roster, index_WK, index_ws, evol1, evol2 int) {
	firstIdentity := roster.List[0]
	wsIdentity := roster.List[index_ws]
	data := make(map[string]*common_structs.WSconfig)
	key := fmt.Sprintf("tls:%v", wsIdentity)
	data[key] = &common_structs.WSconfig{
		ServerID: wsIdentity,
	}
	log.Print("roster is: %s", roster_WK)
	fqdn := fmt.Sprintf("site%d", index_ws)
	id := NewIdentity(roster_WK, fqdn, 1, "CKH_one", "device", nil, data, int64(0))
	err := id.CreateIdentity()
	log.ErrFatal(err)
	log.Print("Site Identity has been created")

	serverIDs := make([]*network.ServerIdentity, 0)
	serverIDs = append(serverIDs, wsIdentity)
	log.Print("Evolution time 1")

	for idx := 0; idx < evol1; idx++ {
		log.Print("evol block: ", idx+1)
		id.ProposeConfig(nil, nil, 0, 0, serverIDs)
		id.ProposeUpVote()
		id.ConfigUpdate()
	}

	client := onet.NewClient("WebServer")
	log.ErrFatal(client.SendProtobuf(wsIdentity, &common_structs.IdentityReady{
		ID:            id.ID,
		Cothority:     roster_WK,
		FirstIdentity: firstIdentity,
		CkhIdentity:   s.ServerIdentity(),
	}, nil))
	log.Print("Cold key holder sent back Identity ready")

	go func() {
		<-s.attachedDone

		log.Print("Evolution time 2 - webservers will have to fetch these blocks from the cothority")

		for idx := 0; idx < evol2; idx++ {
			log.Print("evol block: ", idx+1)
			id.ProposeConfig(nil, nil, 0, 0, serverIDs)
			id.ProposeUpVote()
			id.ConfigUpdate()
		}

		client2 := onet.NewClient(ServiceName)
		log.ErrFatal(client2.SendProtobuf(firstIdentity, &common_structs.MinusOne{s.siteInfoList[0]}, nil))
	}()
}

func startWs(roster, roster_WK *onet.Roster, index_CK, index_ws int) {
	log.Print("startWs")
	wsIdentity := roster.List[index_ws]
	client := onet.NewClient("WebServer")
	log.ErrFatal(client.SendProtobuf(wsIdentity, &common_structs.StartWebserver{
		Roster:    roster,
		Roster_WK: roster_WK,
		Index_CK:  index_CK,
	}, nil))
}

func (s *Service) WaitSetup(roster *onet.Roster, clients, webservers, cothority, evol1, evol2 int) []*common_structs.SiteInfo {
	s.cntMut.Lock()
	s.expected = webservers
	s.cntMut.Unlock()
	msg := &SyncStart{
		Roster:     roster,
		Clients:    clients,
		Webservers: webservers,
		Cothority:  cothority,
		Evol1:      evol1,
		Evol2:      evol2,
	}
	s.SimulFunc(roster, msg, 25000)
	<-s.setupDone
	log.Print(s.Context, "Setup DONE")
	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	return s.siteInfoList
}

func (s *Service) LetsStart(req *common_structs.MinusOne) (network.Message, onet.ClientError) {
	log.Print("FirstIdentity received a MinusOne message")
	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	s.cnt++
	s.siteInfoList = append(s.siteInfoList, req.Sites)
	if s.cnt == s.expected {
		s.setupDone <- true
	}
	return nil, nil
}

func (s *Service) PushedPublic(req *common_structs.PushedPublic) (network.Message, onet.ClientError) {
	log.Print("PushedPublic by the webserver, received by server: ", s.ServerIdentity())
	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	log.Print("before channel receipt")
	s.publicDone <- true
	//s.ok = true
	log.Print("PushedPublic before returning")
	return nil, nil
}

func (s *Service) LetsEvolve(req *common_structs.SiteInfo) (network.Message, onet.ClientError) {
	log.Print("Received a Webserver attached message")
	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	s.siteInfoList = append(s.siteInfoList, req)
	go func() {
		s.attachedDone <- true
	}()
	return nil, nil
}

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

	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/cothority/dns_id/swupdate"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"sort"
	"github.com/dedis/cothority/bftcosi"
	"github.com/satori/go.uuid"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "SIdentity"
const timestamperBFT = "TimestamperBFT"

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
	network.RegisterMessage(&SyncWkhsStart{})
	network.RegisterMessage(&ClientStart{})
	network.RegisterMessage(&WebserverStart{})
	network.RegisterMessage(&common_structs.SiteInfo{})
	network.RegisterMessage(&common_structs.PushedPublic{})
	network.RegisterMessage(common_structs.MinusOne{})
	network.RegisterMessage(common_structs.MinusOneWkh{})
	network.RegisterMessage(common_structs.MinusOneClient{})
	network.RegisterMessage(common_structs.MinusOneWebserver{})
	network.RegisterMessage(common_structs.SetupWkh{})
	network.RegisterMessage(common_structs.StartTimestamper{})
}

// Service handles identities
type Service struct {
	*onet.ServiceProcessor
	skipchain *skipchain.Client
	verifiers map[VerifierID]MsgVerifier
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
	WkhsFunc     messaging.PropagationFunc
	ClientFunc    messaging.PropagationFunc
	WebserverFunc messaging.PropagationFunc
	expected      int
	expected2     int
	expected3     int
	expected4     int
	cnt           int
	cnt2          int
	cnt3          int
	cnt4          int
	cntMut        sync.Mutex
	setupDone     chan bool
	setupWkhsDone     chan bool
	siteInfoList  []*common_structs.SiteInfo
	publicDone    chan bool
	ok            bool
	attachedDone  chan bool
	clientsDone   chan bool
	webserversDone   chan bool
}

// StorageMap holds the map to the storages so it can be marshaled.
type StorageMap struct {
	Identities map[string]*Storage
}

// Storage stores one identity together with the skipblocks.
type Storage struct {
	sync.Mutex
	ID         []byte
	Latest     *common_structs.Config
	Proposed   *common_structs.Config
	Votes      map[string]*crypto.SchnorrSig
	Root       *skipchain.SkipBlock
	Data       *skipchain.SkipBlock
	ConfigBlocks map[string]*common_structs.ConfigPlusNextHash
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
	} else if tn.ProtocolName() == "SyncWkhs" {
		return nil, nil
	} else if tn.ProtocolName() == "StartClients" {
		return nil, nil
	} else if tn.ProtocolName() == "StartWebserverUpt" {
		return nil,nil
	}
	log.Lvlf3("%v: Timestamp Service received New Protocol event", s.String())
	// if CoSi is to be used in the timestamping service
	/*
	pi, err := swupdate.NewCoSiUpdate(tn, dummyVerfier)
	if err != nil {
		log.Lvlf2("%v", err)
	}
	return pi, err
	*/

	return nil, nil
}

/*
 * API messages
 */


func (s *Service) CreateIdentityLight(ai *CreateIdentityLight) (network.Message, onet.ClientError) {
	log.Lvlf2("Request for a new site identity received at server: %v", s.ServerIdentity())
	ids := &Storage{
		Latest: ai.Config,
	}

	ids.ConfigBlocks = make(map[string]*common_structs.ConfigPlusNextHash)
	ids.ID,_ = ids.Latest.Hash()
	roster := ai.Roster

	/*
		// UNCOMMENT IF CAs ARE TO BE USED
		log.Lvlf2("Asking for a cert for site: %v", ids.ID)
		cert, _ := s.ca.SignCert(ai.Config, nil, ids.Data.Hash)
		certinfo := &common_structs.CertInfo{
			Cert:   cert[0],
			SbHash: ids.ID,
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
		SbHash: ids.ID,
	}

	ids.CertInfo = certinfo
	ids.Votes = make(map[string]*crypto.SchnorrSig)


	log.Lvlf2("Propagating Identity")
	replies, err := s.PropagateFunc(roster, &PropagateIdentityLight{ids}, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}


	log.Lvlf2("------CreateIdentity(): Successfully created a new identity-------")
	return &CreateIdentityLightReply{
	}, nil

}


func (s *Storage) setConfigBlock(latestconf *common_structs.Config) bool {
	latestconfHash, _ := latestconf.Hash()
	if s.ConfigBlocks == nil {
		s.ConfigBlocks = make(map[string]*common_structs.ConfigPlusNextHash)
	}
	s.ConfigBlocks[string(latestconfHash)] = &common_structs.ConfigPlusNextHash{
		Config: latestconf,
		NextHash: []byte{0},
	}

	if !bytes.Equal(latestconf.BLink, []byte{0}) {
		s.ConfigBlocks[string(latestconf.BLink)].NextHash = latestconfHash
	}
	return true
}

// getSkipBlockByID returns the skip-block or false if it doesn't exist
func (s *Storage) getConfigBlockByID(sbID []byte) (*common_structs.Config, []byte, bool) {
	//s.Lock()
	//defer s.Unlock()
	value, ok := s.ConfigBlocks[string(sbID)]
	b := value.Config
	hash := value.NextHash
	return b, hash, ok
}

// ProposeSend only stores the proposed configuration internally. Signatures
// come later.
func (s *Service) ProposeSend(p *ProposeSend) (network.Message, onet.ClientError) {
	log.LLvlf2("Received proposal at server: %v", s.ServerIdentity())
	sid := s.getIdentityStorage(p.ID)
	if sid == nil {
		log.LLvlf2("Didn't find Identity")
		return nil, onet.NewClientErrorCode(4100, "Didn't find Identity")
	}

	// Check whether the block points to the current head block, i.e. its BLink is the hash
	// of the latest block
	latestHash, _ := sid.Latest.Hash()
	if !bytes.Equal(p.Config.BLink, latestHash) {
		log.Print("No proper backward link")
		log.Print(p.Config.BLink)
		log.Print(latestHash)
		return nil, onet.NewClientErrorCode(4100, "No proper backward link")
	}
	log.LLvlf2("Storing new proposal 2")
	roster := s.TheRoster
	replies, err := s.PropagateFunc(roster, p, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	log.LLvlf2("Storing new proposal 3 ")
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	log.LLvlf2("Storing new proposal 4")
	return nil, nil
}

func (s *Service) ProposeSendChain(p *ProposeSendChain) (network.Message, onet.ClientError) {
	log.LLvlf2("Received chain-proposal at server: %v", s.ServerIdentity())
	sid := s.getIdentityStorage(p.ID)
	if sid == nil {
		log.LLvlf2("Didn't find Identity")
		return nil, onet.NewClientErrorCode(4100, "Didn't find Identity")
	}

	blocks := p.Blocks
	trustedconfig := sid.Latest.Copy()
	for index, block := range blocks {
		newconfig := block.Copy()
		prevHash, _ := trustedconfig.Hash()
		nextHash, _ := newconfig.Hash()
		log.Lvlf2("Checking trust delegation: %v -> %v (%v -> %v)", index-1, index, prevHash, nextHash)
		cnt := 0
		sig := newconfig.Device["CKH1"].Vote
		if sig != nil {
			err := crypto.VerifySchnorr(network.Suite, newconfig.Device["CKH1"].Point, nextHash, *sig)
			if err != nil {
				log.ErrFatal(err)
			}
			cnt++
		}
		if cnt < trustedconfig.Threshold {
			log.LLvlf2("number of votes: %v, threshold: %v", cnt, trustedconfig.Threshold)
		}
		trustedconfig = newconfig.Copy()
	}



	log.LLvlf2("Propagate the blockchain")
	roster := s.TheRoster
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

// ProposeVote propagates the incoming vote to the rest timestampers upon verification
// of at least one signer as a valid one (to avoid flooding the timestampers, thus mitigating
// DoS attacks)
func (s *Service) ProposeVote(v *ProposeVote) (network.Message, onet.ClientError) {
	log.Lvl2(s, "ProposeVote(): Voting on proposal")
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
	log.Lvl2("Before vote propagation")
	// Propagate the vote
	_, err = s.PropagateFunc(s.TheRoster, v, propagateTimeout)
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	log.Lvl2("After vote propagation")
	return nil, nil

}

// CheckVoteUpdateId takes in account a vote for the proposed config and evolves
// the site identity by updating the latest valid config for the site upon having
// a threshold of signatures
func (s *Service) CheckVoteUpdateId(sid *Storage, v *ProposeVote) (onet.ClientError) {
	log.Lvl2(s, "CheckVoteUpdateId")

	// First verify if the signature is legitimate


	log.Lvl2("Voting on", sid.Proposed.Device)
	owner, ok := sid.Latest.Device[v.Signer]
	if !ok {
		log.Lvlf2("Didn't find signer: %v", v.Signer)
		return onet.NewClientErrorCode(4100,"Didn't find signer")
	}
	if sid.Proposed == nil {
		log.Lvlf2("No proposed block")
		return onet.NewClientErrorCode(4100,"No proposed block")
	}
	hash, err := sid.Proposed.Hash()
	if err != nil {
		log.Lvlf2("Couldn't get hash")
		return onet.NewClientErrorCode(4100,"Couldn't get hash")
	}
	if _, exists := sid.Votes[v.Signer]; exists {
		log.Lvlf2("Already voted for that block")
		return onet.NewClientErrorCode(4100,"Already voted for that block")
	}

	// Check whether our clock is relatively close or not to the proposed timestamp
	err2 := sid.Proposed.CheckTimeDiff(maxdiff_sign)
	if err2 != nil {
		log.Lvlf2("Cothority %v", err2)
		return onet.NewClientError(err2)
	}

	log.Lvl3(v.Signer, "voted", v.Signature)
	if v.Signature != nil {
		err = crypto.VerifySchnorr(network.Suite, owner.Point, hash, *v.Signature)
		if err != nil {
			log.Lvlf2("%v", err)
			return onet.NewClientErrorCode(4100,"Wrong signature: " + err.Error())
		}
	}


	//Store the vote
	log.Lvlf2("Storing vote..")
	log.Lvlf2("Storing vote of signer: %v on proposal..", v.Signer)
	log.Lvlf2("num of votes (without counting our vote): %v", len(sid.Votes))
	if len(sid.Votes) == 0 {
		sid.Votes = make(map[string]*crypto.SchnorrSig)
	}
	sid.Votes[v.Signer] = v.Signature
	sid.Proposed.Device[v.Signer].Vote = v.Signature

	if len(sid.Votes) < sid.Latest.Threshold && len(sid.Votes) != len(sid.Latest.Device) {
		return nil
	}


	// If we have enough signatures, update the site identity
	log.Lvl2("Having at least threshold votes")
	sid.Latest=sid.Proposed.Copy()
	sid.Proposed=nil
	sid.Votes = make(map[string]*crypto.SchnorrSig)
	sid.setConfigBlock(sid.Latest)
	log.Lvl2("identity constituted of", len(sid.ConfigBlocks), "blocks")
	log.Lvl2("hash of latest block: ",hash)
	log.LLvlf2("Server %v has stored the vote", s.ServerIdentity())
	return nil
}

/*
 * Internal messages
 */

// Propagate handles propagation of all data in the identity-service
func (s *Service) Propagate(msg network.Message) {
	log.LLvlf4(" %v got msg %v", s.ServerIdentity(),reflect.TypeOf(msg).String())
	log.Lvlf4(" %v got msg %+v %v", s.ServerIdentity(), msg, reflect.TypeOf(msg).String())
	id := []byte(nil)
	var sid *Storage
	switch msg.(type) {
	case *LockIdentities:
		s.identitiesMutex.Lock()
		return
	case *PushPublicKey:
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
	case *ProposeSendChain:
		id = msg.(*ProposeSendChain).ID
	case *ProposeVote:
		id = msg.(*ProposeVote).ID
	case *PropagateIdentityLight:
		log.Lvlf2("Storing new site identity..")
		pi := msg.(*PropagateIdentityLight)
		id,_ = pi.Latest.Hash()
		if s.getIdentityStorage(id) != nil {
			log.Error("Couldn't store new identity")
			return
		}
		sid = pi.Storage
		sid.Votes = make(map[string]*crypto.SchnorrSig)
		sid.setConfigBlock(pi.Latest)
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
			log.Print("Didn't find entity in", s.ServerIdentity())
			return
		}
		sid.Lock()
		defer sid.Unlock()
		switch msg.(type) {
		case *ProposeSend:
			p := msg.(*ProposeSend)

			// Check whether the block points to the current head block, i.e. its BLink is the hash
			// of the latest block
			latestHash, _ := sid.Latest.Hash()
			if bytes.Equal(p.Config.BLink, latestHash) {
				log.LLvlf2("Storing proposal at server: %v", s.ServerIdentity())
				sid.Proposed = p.Config.Copy()
			} else {
				log.Print("No proper backward link, proposal will not be stored at server: ", s.ServerIdentity())
			}
		case *ProposeSendChain:
			p := msg.(*ProposeSendChain)

			blocks := p.Blocks
			trustedconfig := sid.Latest.Copy()
			for index, block := range blocks {
				newconfig := block.Copy()
				prevHash, _ := trustedconfig.Hash()
				nextHash, _ := newconfig.Hash()
				log.Lvlf2("Checking trust delegation: %v -> %v (%v -> %v)", index-1, index, prevHash, nextHash)
				cnt := 0
				sig := newconfig.Device["CKH1"].Vote
				if sig != nil {
					err := crypto.VerifySchnorr(network.Suite, newconfig.Device["CKH1"].Point, nextHash, *sig)
					if err != nil {
						log.ErrFatal(err)
					}
					cnt++
				}
				if cnt < trustedconfig.Threshold {
					log.LLvlf2("number of votes: %v, threshold: %v", cnt, trustedconfig.Threshold)
				}


				// If we have enough signatures, update the site identity
				log.Lvl2("Having at least threshold votes")
				sid.Latest=newconfig.Copy()
				sid.Proposed=nil
				sid.Votes = make(map[string]*crypto.SchnorrSig)
				sid.setConfigBlock(sid.Latest)

				trustedconfig = newconfig.Copy()
			}

		case *ProposeVote:
			v := msg.(*ProposeVote)
			s.CheckVoteUpdateId(sid, v)
		}
	}
	return
}

// Forward traversal of the skipchain from the oldest block as the latter is
// specified by its hash in the request's 'Hash1' field (if Hash1==[]byte{0}, then start
// fetching from the configblock for the config of which the latest cert is acquired) until
// finding the newest block as it is specified by its hash in the request's 'Hash2' field
// (if Hash2==[]byte{0}, then fetch all configblocks until the current skipchain head one).
// Configblock will be returned from the oldest to the newest
func (s *Service) GetValidSbPathLight(req *GetValidSbPathLight) (network.Message, onet.ClientError) {
	log.Lvl3("Timestamper server received a GetValidSbPathLight message")
	id := req.ID
	h1 := req.Hash1
	h2 := req.Hash2
	sid := s.getIdentityStorage(id)
	if sid == nil {
		log.LLvlf2("Didn't find identity: %v", id)
		return nil, onet.NewClientErrorCode(4100, "Didn't find identity")
	}
	log.Lvlf2("server: %v, site: %v - GetValidSbPath(): Start", s.String(), sid.ID)

	sid.Lock()
	defer sid.Unlock()

	var ok bool
	var sb1 *common_structs.Config
	var nexthash []byte

	if !bytes.Equal(h1, []byte{0}) {
		sb1, _, ok = sid.getConfigBlockByID(h1)
		if !ok {
			log.Print("server: %v, site: %v - NO VALID PATH: Skipblock with hash: %v not found", s.String(), sid.ID, h1)
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
		sb1, _, ok = sid.getConfigBlockByID(h1)
		if !ok {
			log.Print("NO VALID PATH: Skipblock with hash: %v not found", h1)
			return nil, onet.NewClientErrorCode(4100, "NO VALID PATH")
		}
		log.Lvlf3("Last certified skipblock has hash: %v", h1)
	}


	//var sb2 *common_structs.Config
	//sb2 = nil
	if !bytes.Equal(h2, []byte{0}) {
		//sb2, _, ok = site.getConfigBlockByID(h2)
		_, _, ok = sid.getConfigBlockByID(h2)
		if !ok {
			log.Lvlf2("NO VALID PATH: Skipblock with hash: %v not found", h2)
			return nil, onet.NewClientErrorCode(4100,"NO VALID PATH")
		}
	} else {
		// fetch skipblocks until finding the current head of the skipchain
		h2,_ = sid.Latest.Hash()
		//sb2 = sid.Latest
		log.Lvlf3("Current head skipblock has hash: %v", h2)
	}

	oldest := sb1
	//newest := sb2

	log.Lvlf3("Oldest skipblock has hash: %v", h1)
	log.Lvlf3("Newest skipblock has hash: %v", h2)
	sbs := make([]*common_structs.Config, 0)
	block := oldest
	nexthash, _ =block.Hash()
	for !bytes.Equal(nexthash, []byte{0}) {
		temphash := nexthash
		block, nexthash, ok = sid.getConfigBlockByID(temphash)
		if !ok {
			log.Lvlf2("Skipblock with hash: %v not found", temphash)
			return nil, onet.NewClientErrorCode(4100,"Skipblock not found")
		}
		sbs = append(sbs, block)
		log.Lvlf3("Added skipblock with hash: %v, h2: %v, nexthash: %v", temphash, h2, nexthash)
		if bytes.Equal(temphash, h2){
			break
		}
	}


	log.Lvlf2("Timestamper server returns %v blocks, POF: %s", len(sbs), sid.PoF)
	return &GetValidSbPathLightReply{Configblocks: sbs,
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
	hash,_ := sid.Latest.Hash()
	return &GetPoFReply{PoF: pof, SbHash: hash}, nil
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
func (s *Service) getIdentityStorage(id []byte) *Storage {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	is, ok := s.Identities[string(id)]
	if !ok {
		return nil
	}
	return is
}

// setIdentityStorage saves an IdentityStorage
func (s *Service) setIdentityStorage(id []byte, is *Storage) {
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
	log.Print("_______________________________________________________")
	log.Print("------------------TIMESTAMPER BEGINS-------------------")
	log.Print("_______________________________________________________")
	cnt := 0

	for now := range c {
		cnt++
		//data := make([][]byte, 0)
		data2 := make([]common_structs.HashID, 0)
		ids := make([][]byte, 0)
		idsString := make([]string, 0)

		s.identitiesMutex.Lock()
		identities := s.Identities
		s.identitiesMutex.Unlock()

		for _, sid := range identities {
			idsString = append(idsString, string(sid.ID))
		}
		sort.Strings(idsString)
		for _, id := range idsString {
			latestconf := identities[id].Latest
			hash, _ := latestconf.Hash()
			ids = append(ids, hash)
			//data = append(data, hash)
			data2 = append(data2, common_structs.HashID(hash))
			log.LLvlf3("site: %v, %v", latestconf.FQDN, hash)
		}

		num := len(identities)
		if num > 0 {
			log.Lvlf2("_______________________________________________________")
			log.LLvlf2("START OF A TIMESTAMPER ROUND")
			log.Lvlf2("_______________________________________________________")
			log.LLvl2("------- Signing tree root with timestamp:", now, "got", num, "requests")

			// create merkle tree and message to be signed:
			root, proofs := common_structs.ProofTree(sha256.New, data2)
			timestamp := time.Now().Unix() * 1000
			timeB := timestampToBytes(timestamp)
			msg := RecreateSignedMsg(root, timestamp)
			log.Lvlf3("------ Before signing")
			for _, server := range roster.List {
				log.Lvlf3("%v", server)
			}

			// Sign it

			// if CoSi is to be used
			//signature := s.cosiSign(roster, msg)

			// if BFTCoSi is to be used
			bftSignature, err := s.startBFTSignature(roster, msg, timeB)
			if err != nil {
				log.Error(err.Error())
			}
			signature := bftSignature.Sig

			log.Lvlf2("--------- %s: Signed a message.\n", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))

			sids := make([]*Storage, 0)
			for i, idString := range idsString {
				sid := identities[idString]
				pof := &common_structs.SignatureResponse{
					ID:        ids[i],
					Timestamp: timestamp,
					Proof:     proofs[i],
					Root:      root,
					// Collective signature on Timestamp||hash(treeroot)
					Signature:  signature,
					Identifier: cnt,
				}
				/*
				// if CoSi is to be used
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
				origmsg := data[i]
				log.Lvlf3("site: %v", sid.Latest.FQDN)
				log.Lvlf3("%v", []byte(origmsg))
				validproof := pof.Proof.Check(sha256.New, root, []byte(origmsg))
				if !validproof {
					log.Lvlf2("Invalid inclusion proof!")
				}
				*/
				sid.PoF = pof
				sids = append(sids, sid)
			}
			log.Lvlf3("Everything OK with the proofs")
			replies, err := s.PropagateFunc(roster, &PropagatePoF{Storages: sids}, propagateTimeout)
			log.ErrFatal(err, "Couldn't send")
			if replies != len(roster.List) {
				log.Warn("Did only get", replies, "out of", len(roster.List))
			}
			log.Lvlf2("_______________________________________________________")
			log.LLvlf2("END OF A TIMESTAMPER ROUND")
			log.Lvlf2("_______________________________________________________")
			//debug.FreeOSMemory()

		} else {
			log.Lvl3("No follow-sites at epoch:", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
		}
	}
}

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

func (s *Service) startBFTSignature(roster *onet.Roster, msg, timeB []byte) (*bftcosi.BFTSignature, error) {
	log.Lvlf2("server: %s", s.String())
	var bftSignature *bftcosi.BFTSignature
	done := make(chan bool)
	switch len(roster.List) {
	case 0:
		return nil, errors.New("Found empty Roster")
	case 1:
		return nil, errors.New("Need more than 1 entry for Roster")
	}

	// Start the protocol
	tree := roster.GenerateNaryTreeWithRoot(2, s.ServerIdentity())

	node, err := s.CreateProtocol(timestamperBFT, tree)
	if err != nil {
		return nil, errors.New("Couldn't create new node: " + err.Error())
	}

	// Register the function generating the protocol instance
	root := node.(*bftcosi.ProtocolBFTCoSi)
	root.Msg = msg
	root.Data = timeB // timestamp (in bytes) here!!

	// in testing-mode with more than one host and service per cothority-instance
	// we might have the wrong verification-function, so set it again here.
	root.VerificationFunction = s.bftVerify
	// function that will be called when protocol is finished by the root
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	select {
	case <-done:
		log.Lvl2("BFTSignature ready: ", root.Signature())
		bftSignature = root.Signature()
		if len(bftSignature.Exceptions) != 0 {
			log.Print("exceptions=", len(bftSignature.Exceptions))
			return nil, errors.New("Not everybody signed off the new block")
		}
		if err := bftSignature.Verify(network.Suite, roster.Publics()); err != nil {
			return nil, errors.New("Couldn't verify signature")
		}
	case <-time.After(time.Second * 60):
		return nil, errors.New("Timed out while waiting for signature")
	}
	log.Print("startBFTSignature(): Received BFTCoSi response")
	return bftSignature, nil
}


func (s *Service) bftVerify(msgRec, timeB []byte) bool {
	log.Lvlf4("%s verifying msgRec %x", s.ServerIdentity(), msgRec)

	data := make([][]byte, 0)
	data2 := make([]common_structs.HashID, 0)
	ids := make([]string, 0)

	s.identitiesMutex.Lock()
	identities := s.Identities
	s.identitiesMutex.Unlock()

	for _, sid := range identities {
		ids = append(ids, string(sid.ID))
	}
	sort.Strings(ids)
	for _, id := range ids {
		latestconf := identities[id].Latest
		hash, _ := latestconf.Hash()
		data = append(data, hash)
		data2 = append(data2, common_structs.HashID(hash))
		log.Lvlf3("site: %v, %v", latestconf.FQDN, hash)
	}

	// create merkle tree and message that should be signed:
	root, _ := common_structs.ProofTree(sha256.New, []common_structs.HashID(data2))

	msg := make([]byte, len(root)+len(timeB))
	msg = append(msg, root...)
	msg = append(msg, timeB...)

	if !bytes.Equal(msgRec, msg) {
		log.LLvlf2("Receiced msg different from what msg should be signed")
		log.LLvl2(msgRec)
		log.LLvl2(msg)
		return false
	}

	f, ok := s.verifiers[VerifierID(uuid.Nil)]
	if !ok {
		log.LLvlf2("Found no user verification for %x", VerifierID(uuid.Nil))
		return false
	}
	return f(msgRec, nil)
}

// VerifyNoneFunc returns always true.
func (s *Service) VerifyNoneFunc(msg []byte, sb *skipchain.SkipBlock) bool {
	log.Lvl4("No verification - accepted")
	return true
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func (s *Service) RegisterVerification(v VerifierID, f MsgVerifier) error {
	s.verifiers[v] = f
	return nil
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
		StorageMap:       &StorageMap{Identities: make(map[string]*Storage)},
		Publics:          make(map[string]abstract.Point),
		EpochDuration: 	  time.Millisecond * 1000 * 1,
		setupDone:        make(chan bool),
		setupWkhsDone:    make(chan bool),
		publicDone:       make(chan bool),
		attachedDone:     make(chan bool),
		clientsDone:      make(chan bool),
		webserversDone:   make(chan bool),
		verifiers:        map[VerifierID]MsgVerifier{},
	}
	s.ClearIdentities()

	var err error
	s.PropagateFunc, err = messaging.NewPropagationFunc(c, "SIdentityPropagate", s.Propagate)
	log.ErrFatal(err)
	s.SimulFunc, err = messaging.NewPropagationFunc(c, "Sync", s.StartSimul)
	log.ErrFatal(err)
	s.WkhsFunc, err = messaging.NewPropagationFunc(c, "SyncWkhs", s.StartWkhs)
	log.ErrFatal(err)
	s.ClientFunc, err = messaging.NewPropagationFunc(c, "StartClients", s.GoClients)
	log.ErrFatal(err)
	s.WebserverFunc, err = messaging.NewPropagationFunc(c, "StartWebserverUpt", s.GoWebservers)
	log.ErrFatal(err)
	c.ProtocolRegister(timestamperBFT, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerify)
	})
	//ws = c.Service("webserver")
	for _, f := range []interface{}{s.ProposeSend, s.ProposeSendChain, s.ProposeVote, s.SetupWkh,
		s.CreateIdentityLight, s.ProposeUpdate, s.MinusOneWebserver, s.StartTimestamper,
		s.GetValidSbPathLight, s.PushPublicKey, s.PullPublicKey, s.GetCert, s.GetPoF, s.LetsStart, s.LetsStart0,
		s.LetsFinish, s.LetsEvolve, s.PushedPublic,
	} {
		if err := s.RegisterHandler(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}

	if err := s.RegisterVerification(VerifyNone, s.VerifyNoneFunc); err != nil {
		log.Panic(err)
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

type SyncWkhsStart struct {
	Roster     *onet.Roster
	Clients    int
	Webservers int
	Cothority  int
}

type ClientStart struct {
	Roster     *onet.Roster
	Clients    int
	Webservers int
	Cothority  int
	Evol1      int
	Evol2      int
	SiteInfoList []*common_structs.SiteInfo
}

type WebserverStart struct {
	Roster     *onet.Roster
	Clients    int
	Webservers int
}

func (s *Service) GoWebservers(msg network.Message) {
	m := msg.(*WebserverStart)
	// [ clients, webservers, cold key holders, cothority]
	index_client := 0
	index_ws := index_client + m.Clients
	index_CK := index_ws + m.Webservers
	index_WK := index_CK + m.Webservers

	roster_WK := onet.NewRoster(m.Roster.List[index_WK:])

	index, _ := m.Roster.Search(s.Context.ServerIdentity().ID)
	firstIdentity := m.Roster.List[0]

	switch {
	case index < index_ws:
		//client case
		return
	case index < index_CK:
		// webserver case
		go func() {
			s.StartWebserverUpt(m.Roster, index)
			client := onet.NewClient(ServiceName)
			log.ErrFatal(client.SendProtobuf(firstIdentity, &common_structs.MinusOneWebserver{}, nil))
		}()
		return
	case index == index_WK:
		// initializer of the timestamper at the warm key holders
		wkhIdentity := m.Roster.List[index_WK]
		client := onet.NewClient(ServiceName)
		log.ErrFatal(client.SendProtobuf(wkhIdentity, &common_structs.StartTimestamper{roster_WK}, nil))
	default:
		return
	}
}

func (s *Service) GoClients(msg network.Message) {
	m := msg.(*ClientStart)
	// [ clients, webservers, cold key holders, cothority]
	index_client := 0
	index_ws := index_client + m.Clients

	index, _ := m.Roster.Search(s.Context.ServerIdentity().ID)
	firstIdentity := m.Roster.List[0]

	switch {
	case index < index_ws:
		// client case
		go func() {
			var idx int

			if len(m.SiteInfoList) == 1 {
				idx = 0
			} else {
				idx = index%m.Webservers
			}
			s.StartClient(m.Roster, index, m.SiteInfoList[idx : idx+1])
			client := onet.NewClient(ServiceName)
			log.ErrFatal(client.SendProtobuf(firstIdentity, &common_structs.MinusOneClient{}, nil))
		}()
		return
	default:
		return
	}
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
	case index < index_CK:
		// webserver case
		go startWs(m.Roster, roster_WK, index_CK+(index-index_ws), index)
	case index < index_WK:
		// cold key case
		go func() {
			<-s.publicDone
			log.Print(s.Context, "ColdKeyHolder is now ready to pursue")
			randWkh := index_WK + rand.Int()%(len(m.Roster.List)-index_WK)
			s.startCkh(m.Roster, roster_WK, randWkh, index_ws+(index-index_CK), m.Evol1, m.Evol2)
		}()
	default:
		// cothority warm key case
	}
	return
}

func (s *Service) StartWkhs(msg network.Message) {
	log.Print("StartWkhs", s.Context.String())
	m := msg.(*SyncWkhsStart)

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
	case index < index_CK:
		// webserver case
	case index < index_WK:
		// cold key case
	default:
		// cothority warm key case
		go s.startWkh(m.Roster, roster_WK, index)
	}
	return
}


func (s *Service) startWkh(roster, roster_WK *onet.Roster, index_WK int) {
	wkhIdentity := roster.List[index_WK]
	client := onet.NewClient(ServiceName)
	log.ErrFatal(client.SendProtobuf(wkhIdentity, &common_structs.SetupWkh{roster, roster_WK}, nil))
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
	ids, err := NewIdentityMultDevs(roster_WK, fqdn, 1, []string{"CKH1", "CKH2", "CKH3", "CKH4", "CKH5"}, "device", data)
	if err != nil {
		log.ErrFatal(err)
	}
	id := ids[0]
	err = id.CreateIdentityLight()
	log.ErrFatal(err)
	log.Print("Site Identity has been created")

	serverIDs := make([]*network.ServerIdentity, 0)
	serverIDs = append(serverIDs, wsIdentity)
	log.Print("Evolution time 1")
	id.EvolveChain(nil, nil, 0,  serverIDs, evol1)
	/*
	for idx := 0; idx < evol1; idx++ {
		log.Print("evol block: ", idx+1, "server: ", s.ServerIdentity())
		id.ProposeConfig(nil, nil, 0,  serverIDs)
		log.Print("before ProposeUpVote() starts")
		id.ProposeUpVote()
	}
	*/
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
			id.ProposeConfig(nil, nil, 0,  serverIDs)
			id.ProposeUpVote()
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

func (s *Service) StartClient(roster *onet.Roster, index_client int, info []*common_structs.SiteInfo) {
	log.Print("startClient")
	clientIdentity := roster.List[index_client]
	client := onet.NewClient("WebServer")
	log.ErrFatal(client.SendProtobuf(clientIdentity, &common_structs.ConnectClient{info}, nil))
}

func (s *Service) StartWebserverUpt(roster *onet.Roster, index_ws int) {
	log.Print("startWebserverUpdates")
	wsIdentity := roster.List[index_ws]
	client := onet.NewClient("WebServer")
	log.ErrFatal(client.SendProtobuf(wsIdentity, &common_structs.StartUptWebserver{}, nil))
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


func (s *Service) WaitSetupWkhs(roster *onet.Roster, clients, webservers, cothority int)  {
	s.cntMut.Lock()
	s.expected4 = cothority
	s.cntMut.Unlock()
	msg := &SyncWkhsStart{
		Roster:     roster,
		Clients:    clients,
		Webservers: webservers,
		Cothority:  cothority,
	}
	s.WkhsFunc(roster, msg, 25000)
	<-s.setupWkhsDone
	log.Print(s.Context, "SetupWkhs DONE")
	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	return
}
func (s *Service) WaitClients(roster *onet.Roster, clients, webservers, cothority, evol1, evol2 int, siteinfolist []*common_structs.SiteInfo) {
	s.cntMut.Lock()
	s.expected2 = clients
	s.cntMut.Unlock()

	msg := &ClientStart{
		Roster:     roster,
		Clients:    clients,
		Webservers: webservers,
		Cothority:  cothority,
		Evol1:      evol1,
		Evol2:      evol2,
		SiteInfoList: siteinfolist,
	}

	s.ClientFunc(roster, msg, 25000)
	go func() {
		<-s.clientsDone
		log.Print(s.Context, "Clients DONE")
		return
	}()
}


func (s *Service) WaitWebservers(roster *onet.Roster, clients, webservers int) {
	s.cntMut.Lock()
	s.expected3 = webservers
	s.cntMut.Unlock()

	msg := &WebserverStart{
		Roster:     roster,
		Clients:    clients,
		Webservers: webservers,
	}

	s.WebserverFunc(roster, msg, 25000)
	go func() {
		<-s.webserversDone
		log.Print(s.Context, "Webservers DONE")
		return
	}()
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

func (s *Service) LetsStart0(req *common_structs.MinusOneWkh) (network.Message, onet.ClientError) {

	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	s.cnt4++
	log.Print("FirstIdentity received a MinusOneWkh message, (in total:", s.cnt4, "out of:", s.expected4, ")")
	if s.cnt4 == s.expected4 {
		s.setupWkhsDone <- true
	}
	return nil, nil
}

func (s *Service) LetsFinish(req *common_structs.MinusOneClient) (network.Message, onet.ClientError) {
	log.Print("FirstIdentity received a MinusOneClient message")
	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	s.cnt2++
	log.Printf("clients already finished visiting website: %v",s.cnt2)
	if s.cnt2 == s.expected2 {
		s.clientsDone <- true
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

func (s *Service) MinusOneWebserver(req *common_structs.MinusOneWebserver) (network.Message, onet.ClientError) {
	log.Print("FirstIdentity received a MinusOneWebserver message")
	s.cntMut.Lock()
	defer s.cntMut.Unlock()
	s.cnt3++
	log.Printf("%v",s.cnt3)
	if s.cnt3 == s.expected3 {
		s.webserversDone <- true
	}
	return nil, nil
}

func (s *Service) SetupWkh(req *common_structs.SetupWkh) (network.Message, onet.ClientError) {
	roster := req.Roster
	firstIdentity := roster.List[0]
	roster_WK :=  req.Wkhs
	s.identitiesMutex.Lock()
	s.TheRoster=roster_WK
	s.identitiesMutex.Unlock()
	client := onet.NewClient(ServiceName)
	log.ErrFatal(client.SendProtobuf(firstIdentity, &common_structs.MinusOneWkh{}, nil))
	return nil, nil
}


func (s *Service) StartTimestamper(req *common_structs.StartTimestamper) (network.Message, onet.ClientError) {
	roster_WK :=  req.Wkhs
	go s.RunLoop(roster_WK)
	return nil, nil
}
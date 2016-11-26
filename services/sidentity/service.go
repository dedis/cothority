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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sync"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "SIdentity"

var IdentityService sda.ServiceID

func init() {
	sda.RegisterNewService(ServiceName, newIdentityService)
	IdentityService = sda.ServiceFactory.ServiceID(ServiceName)
	network.RegisterPacketType(&StorageMap{})
	network.RegisterPacketType(&Storage{})
}

// Service handles identities
type Service struct {
	*sda.ServiceProcessor
	ca *ca.CSRDispatcher
	*StorageMap
	identitiesMutex sync.Mutex
	skipchain       *skipchain.Client
	path            string
}

// StorageMap holds the map to the storages so it can be marshaled.
type StorageMap struct {
	Identities map[string]*Storage
}

// Storage stores one identity together with the skipblocks.
type Storage struct {
	sync.Mutex
	Latest     *common_structs.Config
	Proposed   *common_structs.Config
	Votes      map[string]*crypto.SchnorrSig
	Root       *skipchain.SkipBlock
	Data       *skipchain.SkipBlock
	SkipBlocks map[string]*skipchain.SkipBlock
	//Certs      []*ca.Cert
	// Certs2 keeps the mapping between the config (the hash of the skipblock that contains it) and the cert(s)
	// that was(were) issued for that particular config
	//Certs  []*ca.Cert
	Certs map[string][]*ca.Cert
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl3(s.ServerIdentity(), "Identity received New Protocol event", conf)
	switch tn.ProtocolName() {
	case "Propagate":
		pi, err := manage.NewPropagateProtocol(tn)
		if err != nil {
			return nil, err
		}
		pi.(*manage.Propagate).RegisterOnData(s.Propagate)
		return pi, err
	}
	return nil, nil
}

/*
 * API messages
 */

// CreateIdentity will register a new SkipChain and add it to our list of
// managed identities.
func (s *Service) CreateIdentity(si *network.ServerIdentity, ai *CreateIdentity) (network.Body, error) {
	log.Lvlf3("%s Creating new identity with config %+v", s, ai.Config)
	ids := &Storage{
		Latest: ai.Config,
	}
	log.Lvl3("Creating Root-skipchain")
	var err error
	ids.Root, err = s.skipchain.CreateRoster(ai.Roster, 2, 10,
		skipchain.VerifyNone, nil)
	if err != nil {
		return nil, err
	}
	log.Lvl3("Creating Data-skipchain")
	ids.Root, ids.Data, err = s.skipchain.CreateData(ids.Root, 2, 10,
		skipchain.VerifyNone, ai.Config)
	if err != nil {
		return nil, err
	}

	ids.SkipBlocks = make(map[string]*skipchain.SkipBlock)
	ids.setSkipBlockByID(ids.Data)

	roster := ids.Root.Roster

	certs, _ := s.ca.SignCert(ai.Config, ids.Data.Hash)
	if certs == nil {
		log.Lvlf2("No certs returned")
	}

	ids.Certs = make(map[string][]*ca.Cert)
	hash := ids.Data.Hash
	for _, cert := range certs {
		slice := ids.Certs[string(hash)]
		slice = append(slice, cert)
		ids.Certs[string(hash)] = slice

		//ids.Certs = append(ids.Certs, cert)
		log.Lvlf2("---------NEW CERT!--------")
		log.Lvlf2("siteID: %v, hash: %v, sig: %v, public: %v", cert.ID, cert.Hash, cert.Signature, cert.Public)
	}

	replies, err := manage.PropagateStartAndWait(s.Context, roster,
		&PropagateIdentity{ids}, propagateTimeout, s.Propagate)
	if err != nil {
		return nil, err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	log.Lvlf2("New chain is\n%x", []byte(ids.Data.Hash))

	s.save()
	//log.Lvlf2("CreateIdentity(): End having %v certs", len(ids.Certs))
	cnt := 0
	for _, certarray := range ids.Certs {
		for _, _ = range certarray {
			cnt++
		}
	}
	log.Lvlf2("CreateIdentity(): End having %v certs", cnt)
	return &CreateIdentityReply{
		Root: ids.Root,
		Data: ids.Data,
	}, nil
}

// ConfigUpdate returns a new configuration update
func (s *Service) ConfigUpdate(si *network.ServerIdentity, cu *ConfigUpdate) (network.Body, error) {
	sid := s.getIdentityStorage(cu.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	sid.Lock()
	defer sid.Unlock()
	log.Lvl3(s, "Sending config-update")
	return &ConfigUpdateReply{
		Config: sid.Latest,
	}, nil
}

func (s *Storage) setSkipBlockByID(latest *skipchain.SkipBlock) bool {
	s.SkipBlocks[string(latest.Hash)] = latest
	return true
}

// getSkipBlockByID returns the skip-block or false if it doesn't exist
func (s *Storage) getSkipBlockByID(sbID skipchain.SkipBlockID) (*skipchain.SkipBlock, bool) {
	b, ok := s.SkipBlocks[string(sbID)]
	//b, ok := s.SkipBlocks["georgia"]
	return b, ok
}

// forward traversal of the skipchain
func (s *Service) GetUpdateChain(si *network.ServerIdentity, latestKnown *GetUpdateChain) (network.Body, error) {
	sid := s.getIdentityStorage(latestKnown.ID)
	//log.Lvlf2("GetUpdateChain(): Start having %v certs", len(sid.Certs))
	cnt := 0
	certs := make([]*ca.Cert, 0)
	for _, certarray := range sid.Certs {
		for _, cert := range certarray {
			cnt++
			certs = append(certs, cert)
		}
	}
	log.LLvlf2("GetUpdateChain(): Start having %v certs", cnt)
	log.LLvlf2("GetUpdateChain(): Latest known block has hash: %v", latestKnown.LatestID)
	block, ok := sid.getSkipBlockByID(latestKnown.LatestID)
	if !ok {
		return nil, errors.New("Couldn't find latest skipblock!!")
	}

	blocks := []*skipchain.SkipBlock{block}
	log.Lvl3("Starting to search chain")
	for len(block.ForwardLink) > 0 {
		//link := block.ForwardLink[len(block.ForwardLink)-1]
		// for linear forward traversal of the skipchain:
		link := block.ForwardLink[0]
		hash := link.Hash
		block, ok = sid.getSkipBlockByID(hash)
		if !ok {
			return nil, errors.New("Missing block in forward-chain")
		}
		blocks = append(blocks, block)
		//fmt.Println("another block found with hash: ", skipchain.SkipBlockID(hash))
	}
	log.LLvlf2("Found %v blocks", len(blocks))
	for index, block := range blocks {
		log.LLvlf2("block: %v with hash: %v", index, block.Hash)
	}

	cnt = 0
	certs = make([]*ca.Cert, 0)
	for _, certarray := range sid.Certs {
		for _, cert := range certarray {
			cnt++
			certs = append(certs, cert)
		}
	}
	log.Lvlf2("GetUpdateChain(): End having %v certs", cnt)
	reply := &GetUpdateChainReply{
		Update: blocks,
		Certs:  certs,
	}
	return reply, nil
}

// ProposeSend only stores the proposed configuration internally. Signatures
// come later.
func (s *Service) ProposeSend(si *network.ServerIdentity, p *ProposeSend) (network.Body, error) {
	log.LLvlf2("Storing new proposal")
	sid := s.getIdentityStorage(p.ID)
	if sid == nil {
		log.Lvlf2("Didn't find Identity")
		return nil, errors.New("Didn't find Identity")
	}
	roster := sid.Root.Roster
	replies, err := manage.PropagateStartAndWait(s.Context, roster,
		p, propagateTimeout, s.Propagate)
	if err != nil {
		return nil, err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil, nil
}

// ProposeUpdate returns an eventual config-proposition
func (s *Service) ProposeUpdate(si *network.ServerIdentity, cnc *ProposeUpdate) (network.Body, error) {
	log.Lvl3(s, "Sending proposal-update to client")
	sid := s.getIdentityStorage(cnc.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
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
func (s *Service) ProposeVote(si *network.ServerIdentity, v *ProposeVote) (network.Body, error) {
	log.Lvl2(s, "Voting on proposal")
	// First verify if the signature is legitimate
	sid := s.getIdentityStorage(v.ID)
	if sid == nil {
		return nil, errors.New("Didn't find identity")
	}

	// Putting this in a function because of the lock which needs to be held
	// over all calls that might return an error.
	err := func() error {
		sid.Lock()
		defer sid.Unlock()
		log.Lvl3("Voting on", sid.Proposed.Device)
		owner, ok := sid.Latest.Device[v.Signer]
		if !ok {
			return errors.New("Didn't find signer")
		}
		if sid.Proposed == nil {
			return errors.New("No proposed block")
		}
		hash, err := sid.Proposed.Hash()
		if err != nil {
			return errors.New("Couldn't get hash")
		}
		if _, exists := sid.Votes[v.Signer]; exists {
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
				return errors.New("Wrong signature: " + err.Error())
			}
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	// Propagate the vote
	_, err = manage.PropagateStartAndWait(s.Context, sid.Root.Roster, v, propagateTimeout, s.Propagate)
	if err != nil {
		return nil, err
	}
	if len(sid.Votes) >= sid.Latest.Threshold ||
		len(sid.Votes) == len(sid.Latest.Device) {
		// If we have enough signatures, make a new data-skipblock and
		// propagate it
		log.Lvl3("Having majority or all votes")

		// Making a new data-skipblock
		log.Lvl3("Sending data-block with", sid.Proposed.Device)
		reply, err := s.skipchain.ProposeData(sid.Root, sid.Data, sid.Proposed)
		if err != nil {
			return nil, err
		}
		_, msg, _ := network.UnmarshalRegistered(reply.Latest.Data)
		log.Lvl3("SB signed is", msg.(*common_structs.Config).Device)
		usb := &UpdateSkipBlock{
			ID:       v.ID,
			Latest:   reply.Latest,
			Previous: reply.Previous,
		}
		sid.setSkipBlockByID(usb.Latest)
		sid.setSkipBlockByID(usb.Previous)
		_, err = manage.PropagateStartAndWait(s.Context, sid.Root.Roster,
			usb, propagateTimeout, s.Propagate)
		if err != nil {
			return nil, err
		}
		s.save()
		//fmt.Println("latest block's hash: ", sid.Data.Hash, "number of flinks: ", len(sid.Data.ForwardLink))
		//fmt.Println(sid.Data.BackLinkIds[0])
		//block1, _ := sid.getSkipBlockByID(ID(sid.Data.BackLinkIds[0]))
		//fmt.Println(len(block1.ForwardLink))
		//fmt.Println("latest block's hash: ", usb.Latest.Hash)
		return &ProposeVoteReply{sid.Data}, nil
	}
	return nil, nil
}

/*
 * Internal messages
 */

// Propagate handles propagation of all data in the identity-service
func (s *Service) Propagate(msg network.Body) {
	log.Lvlf4("Got msg %+v %v", msg, reflect.TypeOf(msg).String())
	id := skipchain.SkipBlockID(nil)
	switch msg.(type) {
	/*case *ProposeCert:
	id = msg.(*ProposeCert).Cert.ID*/
	case *ProposeSend:
		id = msg.(*ProposeSend).ID
	case *ProposeVote:
		id = msg.(*ProposeVote).ID
	case *UpdateSkipBlock:
		id = msg.(*UpdateSkipBlock).ID
	case *PropagateIdentity:
		pi := msg.(*PropagateIdentity)
		id = pi.Data.Hash
		if s.getIdentityStorage(id) != nil {
			log.Error("Couldn't store new identity")
			return
		}
		log.Lvl3("Storing identity in", s)
		s.setIdentityStorage(id, pi.Storage)

		sid := s.getIdentityStorage(id)
		sid.SkipBlocks = make(map[string]*skipchain.SkipBlock)
		sid.setSkipBlockByID(pi.Data)
		return
	}

	if id != nil {
		sid := s.getIdentityStorage(id)
		if sid == nil {
			log.Error("Didn't find entity in", s)
			return
		}
		sid.Lock()
		defer sid.Unlock()
		switch msg.(type) {
		case *ProposeSend:
			p := msg.(*ProposeSend)
			sid.Proposed = p.Config
			sid.Votes = make(map[string]*crypto.SchnorrSig)
		case *ProposeVote:
			v := msg.(*ProposeVote)
			sid.Votes[v.Signer] = v.Signature
			sid.Proposed.Device[v.Signer].Vote = v.Signature
		case *UpdateSkipBlock:
			skipblock_previous := msg.(*UpdateSkipBlock).Previous
			skipblock_latest := msg.(*UpdateSkipBlock).Latest
			_, msgLatest, err := network.UnmarshalRegistered(skipblock_latest.Data)
			if err != nil {
				log.Error(err)
				return
			}
			al, ok := msgLatest.(*common_structs.Config)
			if !ok {
				log.Error(err)
				return
			}
			sid.Data = skipblock_latest
			sid.Latest = al
			sid.Proposed = nil
			//sid.SkipBlocks[string(skipblock.Hash)] = skipblock
			sid.setSkipBlockByID(skipblock_latest)
			sid.setSkipBlockByID(skipblock_previous)
			//fmt.Println("hash: ", skipblock_latest.Hash)
		}
	}
}

// backward traversal of the skipchain until finding a skipblock whose config has been certified
// (a cert has been issued for it)
func (s *Service) GetSkipblocks(si *network.ServerIdentity, req *GetSkipblocks) (network.Body, error) {
	log.LLvlf2("GetSkipblocks(): Start")
	id := req.ID
	latest := req.Latest
	sid := s.getIdentityStorage(id)
	if sid == nil {
		log.LLvlf2("Didn't find identity")
		return nil, errors.New("Didn't find identity")
	}
	// Follow the backward links until finding the skipblock whose config was certified by a CA
	// All these skipblocks will be returned (from the oldest to the newest)
	sbs := make([]*skipchain.SkipBlock, 1)
	//sbs = append(sbs, latest)
	block := latest
	hash := block.Hash
	var ok bool
	_, exists := sid.Certs[string(hash)]
	for {
		block, ok = sid.getSkipBlockByID(hash)
		//log.LLvlf2("hash: %v", block.Hash)
		if !ok {
			log.LLvlf2("Skipblock with hash: %v not found", hash)
			return nil, fmt.Errorf("Skipblock with hash: %v not found", hash)
		}
		sbs = append(sbs, block)
		if exists {
			break
		}
		hash = block.BackLinkIds[0]
		_, exists = sid.Certs[string(hash)]
	}

	sbs_from_oldest := make([]*skipchain.SkipBlock, len(sbs))
	for index, block := range sbs {
		sbs_from_oldest[len(sbs)-1-index] = block
	}
	log.LLvlf2("GetSkipblocks(): End with %v blocks to return", len(sbs))
	log.LLvlf2("GetSkipblocks(): End with %v blocks to return", len(sbs_from_oldest))
	return &GetSkipblocksReply{Skipblocks: sbs_from_oldest}, nil
}

// Forward traversal of the skipchain from the oldest block as the latter is
// specified in the request's 'Sb1' field until finding the newest block as it
// is specified in the request's 'Sb2' field (if Sb2==nil, then set Sb2 as the current
// skipchain head). Skipblocks will be returned from the oldest to the newest
func (s *Service) GetValidSbPath(si *network.ServerIdentity, req *GetValidSbPath) (network.Body, error) {
	log.LLvlf2("GetValidSbPath(): Start")
	id := req.ID
	sb1 := req.Sb1
	sb2 := req.Sb2
	sid := s.getIdentityStorage(id)
	if sid == nil {
		log.LLvlf2("Didn't find identity")
		return nil, errors.New("Didn't find identity")
	}
	_, ok := sid.getSkipBlockByID(sb1.Hash)
	if !ok {
		log.LLvlf2("NO VALID PATH: Skipblock with hash: %v not found", sb1.Hash)
		return nil, fmt.Errorf("NO VALID PATH: Skipblock with hash: %v not found", sb1.Hash)
	}

	if sb2 != nil {
		_, ok := sid.getSkipBlockByID(sb2.Hash)
		if !ok {
			log.LLvlf2("NO VALID PATH: Skipblock with hash: %v not found", sb2.Hash)
			return nil, fmt.Errorf("NO VALID PATH: Skipblock with hash: %v not found", sb2.Hash)
		}
	} else {
		// in this case, fetch all the blocks starting from the latest known one (sb1)
		// till the current head of the skipchain
		sb2 = sid.Data
		log.LLvlf2("Current head skipblock has hash: %v", sb2.Hash)
	}

	oldest := sb1
	newest := sb2
	/*
		_, data, _ := network.UnmarshalRegistered(sb1.Data)
		conf1, _ := data.(*common_structs.Config)

		_, data, _ = network.UnmarshalRegistered(sb2.Data)
		conf2, _ := data.(*common_structs.Config)

		newest := sb1
		oldest := sb2
		is_older := conf1.IsOlderConfig(conf2)
		log.LLvl2(is_older)
		if is_older {
			log.LLvlf2("Swapping blocks")
			newest = sb2
			oldest = sb1
		}
	*/
	log.LLvlf2("Oldest skipblock has hash: %v", oldest.Hash)
	log.LLvlf2("Newest skipblock has hash: %v", newest.Hash)
	sbs := make([]*skipchain.SkipBlock, 0)
	sbs = append(sbs, oldest)
	block := oldest
	log.LLvlf2("Skipblock with hash: %v", block.Hash)
	for len(block.ForwardLink) > 0 {
		link := block.ForwardLink[0]
		hash := link.Hash
		log.LLvlf2("Appending skipblock with hash: %v", hash)
		block, ok = sid.getSkipBlockByID(hash)
		if !ok {
			log.LLvlf2("Skipblock with hash: %v not found", hash)
			return nil, fmt.Errorf("Skipblock with hash: %v not found", hash)
		}
		sbs = append(sbs, block)
		if bytes.Equal(hash, sid.Data.Hash) || bytes.Equal(hash, newest.Hash) {
			break
		}
	}

	log.LLvlf2("Num of returned blocks: %v", len(sbs))
	return &GetValidSbPathReply{Skipblocks: sbs}, nil
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
	b, err := network.MarshalRegisteredType(s.StorageMap)
	if err != nil {
		log.Error("Couldn't marshal service:", err)
	} else {
		err = ioutil.WriteFile(s.path+"/sidentity.bin", b, 0660)
		if err != nil {
			log.Error("Couldn't save file:", err)
		}
	}
}

func (s *Service) clearIdentities() {
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
		_, msg, err := network.UnmarshalRegistered(b)
		if err != nil {
			return fmt.Errorf("Couldn't unmarshal: %s", err)
		}
		log.Lvl3("Successfully loaded")
		s.StorageMap = msg.(*StorageMap)
	}
	return nil
}

func newIdentityService(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		ca:               ca.NewCSRDispatcher(),
		StorageMap:       &StorageMap{make(map[string]*Storage)},
		skipchain:        skipchain.NewClient(),
		path:             path,
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	for _, f := range []interface{}{s.ProposeSend, s.ProposeVote,
		s.CreateIdentity, s.ProposeUpdate, s.ConfigUpdate, s.GetUpdateChain,
		s.GetSkipblocks, s.GetValidSbPath,
	} {
		if err := s.RegisterMessage(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return s
}

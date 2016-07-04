package identity

/*
SSH-ks is a keystorage service for SSH keys. You can have a set of clients
that communicate with a cothority to keep the list of public keys updated.
A number of servers track all changes and update their .authorized_hosts
accordingly.
*/

import (
	"errors"

	"sync"

	"reflect"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Identity"

var identityService sda.ServiceID

func init() {
	sda.RegisterNewService(ServiceName, newIdentityService)
	identityService = sda.ServiceFactory.ServiceID(ServiceName)
}

// Service handles identities
type Service struct {
	*sda.ServiceProcessor
	identities      map[string]*Storage
	identitiesMutex sync.Mutex
	skipchain       *skipchain.Client
	path            string
}

// Storage stores one identity together with the skipblocks
type Storage struct {
	sync.Mutex
	Latest   *AccountList
	Proposed *AccountList
	Votes    map[string]*crypto.SchnorrSig
	Root     *skipchain.SkipBlock
	Data     *skipchain.SkipBlock
}

// AddIdentity will register a new SkipChain and add it to our list of
// managed identities
func (s *Service) AddIdentity(e *network.ServerIdentity, ai *AddIdentity) (network.Body, error) {
	log.Lvlf2("Adding identity %+v", *ai)
	ids := &Storage{
		Latest: ai.AccountList,
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
		skipchain.VerifyNone, ai.AccountList)
	if err != nil {
		return nil, err
	}

	roster := ids.Root.Roster
	replies, err := manage.PropagateStartAndWait(s.Context, roster,
		&PropagateIdentity{ids}, 1000, s.Propagate)
	if err != nil {
		return nil, err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}

	return &AddIdentityReply{
		Root: ids.Root,
		Data: ids.Data,
	}, nil
}

// ProposeConfig only stores the proposed configuration internally. Signatures
// come later.
func (s *Service) ProposeConfig(e *network.ServerIdentity, p *PropagateProposition) (network.Body, error) {
	sid := s.getIdentityStorage(p.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	roster := sid.Root.Roster
	replies, err := manage.PropagateStartAndWait(s.Context, roster,
		p, 1000, s.Propagate)
	if err != nil {
		return nil, err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil, nil
}

// ConfigNewCheck returns an eventual config-proposition
func (s *Service) ConfigNewCheck(e *network.ServerIdentity, cnc *ConfigNewCheck) (network.Body, error) {
	sid := s.getIdentityStorage(cnc.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	sid.Lock()
	defer sid.Unlock()
	return &ConfigNewCheck{
		ID:          cnc.ID,
		AccountList: sid.Proposed,
	}, nil
}

// ConfigUpdate returns a new configuration update
func (s *Service) ConfigUpdate(e *network.ServerIdentity, cu *ConfigUpdate) (network.Body, error) {
	sid := s.getIdentityStorage(cu.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	sid.Lock()
	defer sid.Unlock()
	return &ConfigUpdate{
		ID:          cu.ID,
		AccountList: sid.Latest,
	}, nil
}

// VoteConfig takes int account a vote for the proposed config. It also verifies
// that the voter is in the latest config.
// An empty signature signifies that the vote has been rejected.
func (s *Service) VoteConfig(e *network.ServerIdentity, v *Vote) (network.Body, error) {
	// First verify if the signature is legitimate
	sid := s.getIdentityStorage(v.ID)
	if sid == nil {
		return nil, errors.New("Didn't find identity")
	}
	sid.Lock()
	log.Lvl3("Voting on", sid.Proposed.Owners)
	owner, ok := sid.Latest.Owners[v.Signer]
	if !ok {
		return nil, errors.New("Didn't find signer")
	}
	if sid.Proposed == nil {
		return nil, errors.New("No proposed block")
	}
	hash, err := sid.Proposed.Hash()
	if err != nil {
		return nil, errors.New("Couldn't get hash")
	}
	if _, exists := sid.Votes[v.Signer]; exists {
		return nil, errors.New("Already voted for that block")
	}
	log.Lvl3(v.Signer, "voted", v.Signature)
	if v.Signature != nil {
		err = crypto.VerifySchnorr(network.Suite, owner.Point, hash, *v.Signature)
		if err != nil {
			return nil, errors.New("Wrong signature: " + err.Error())
		}
	}
	sid.Unlock()

	// Propagate the vote
	_, err = manage.PropagateStartAndWait(s.Context, sid.Root.Roster, v, 1000, s.Propagate)
	if err != nil {
		return nil, err
	}
	if len(sid.Votes) >= sid.Latest.Threshold ||
		len(sid.Votes) == len(sid.Latest.Owners) {
		// If we have enough signatures, make a new data-skipblock and
		// propagate it
		log.Lvl3("Having majority or all votes")

		// Making a new data-skipblock
		log.Lvl3("Sending data-block with", sid.Proposed.Owners)
		reply, err := s.skipchain.ProposeData(sid.Root, sid.Data, sid.Proposed)
		if err != nil {
			return nil, err
		}
		_, msg, _ := network.UnmarshalRegistered(reply.Latest.Data)
		log.Lvl3("SB signed is", msg.(*AccountList).Owners)
		usb := &UpdateSkipBlock{
			ID:     v.ID,
			Latest: reply.Latest,
		}
		_, err = manage.PropagateStartAndWait(s.Context, sid.Root.Roster,
			usb, 1000, s.Propagate)
		if err != nil {
			return nil, err
		}
		return sid.Data, nil
	}
	return nil, nil
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl3(s.ServerIdentity(), "Identity received New Protocol event", tn.ProtocolName(), tn, conf)
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

// Propagate handles propagation of all data in the identity-service
func (s *Service) Propagate(msg network.Body) {
	log.Lvlf4("Got msg %+v %v", msg, reflect.TypeOf(msg).String())
	id := ID(nil)
	switch msg.(type) {
	case *PropagateProposition:
		id = msg.(*PropagateProposition).ID
	case *Vote:
		id = msg.(*Vote).ID
	case *UpdateSkipBlock:
		id = msg.(*UpdateSkipBlock).ID
	case *PropagateIdentity:
		pi := msg.(*PropagateIdentity)
		id = ID(pi.Data.Hash)
		if s.getIdentityStorage(id) != nil {
			log.Error("Couldn't store new identity")
			return
		}
		log.Lvl3("Storing identity in", s)
		s.setIdentityStorage(id, pi.Storage)
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
		case *PropagateProposition:
			p := msg.(*PropagateProposition)
			sid.Proposed = p.AccountList
			sid.Votes = make(map[string]*crypto.SchnorrSig)
		case *Vote:
			v := msg.(*Vote)
			sid.Votes[v.Signer] = v.Signature
		case *UpdateSkipBlock:
			skipblock := msg.(*UpdateSkipBlock).Latest
			_, msgLatest, err := network.UnmarshalRegistered(skipblock.Data)
			if err != nil {
				log.Error(err)
				return
			}
			al, ok := msgLatest.(*AccountList)
			if !ok {
				log.Error(err)
				return
			}
			sid.Data = skipblock
			sid.Latest = al
			sid.Proposed = nil
		}
	}
}

// getIdentityStorage returns the corresponding IdentityStorage or nil
// if none was found
func (s *Service) getIdentityStorage(id ID) *Storage {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	is, ok := s.identities[string(id)]
	if !ok {
		return nil
	}
	return is
}

// setIdentityStorage saves an IdentityStorage
func (s *Service) setIdentityStorage(id ID, is *Storage) {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	log.Lvlf3("%s %x %v", s.Context.ServerIdentity(), id[0:8], is.Latest.Owners)
	s.identities[string(id)] = is
}

func newIdentityService(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		identities:       make(map[string]*Storage),
		skipchain:        skipchain.NewClient(),
		path:             path,
	}
	for _, f := range []interface{}{s.ProposeConfig, s.VoteConfig,
		s.AddIdentity, s.ConfigNewCheck, s.ConfigUpdate} {
		if err := s.RegisterMessage(f); err != nil {
			log.Fatal("Registration error:", err)
		}
	}
	return s
}

package identity

/*
SSH-ks is a keystorage service for SSH keys. You can have a set of clients
that communicate with a cothority to keep the list of public keys updated.
A number of servers track all changes and update their .authorized_hosts
accordingly.
*/

import (
	"errors"
	"strings"

	"sync"

	"reflect"

	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/manage"
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
	identities      map[string]*IdentityStorage
	identitiesMutex sync.Mutex
	skipchain       *skipchain.Client
	path            string
}

type IdentityStorage struct {
	sync.Mutex
	Latest   *AccountList
	Proposed *AccountList
	Votes    map[string]*crypto.SchnorrSig
	Root     *skipchain.SkipBlock
	Data     *skipchain.SkipBlock
}

// AddIdentity will register a new SkipChain and add it to our list of
// managed identities
func (s *Service) AddIdentity(e *network.Entity, ai *AddIdentity) (network.ProtocolMessage, error) {
	ids := &IdentityStorage{
		Latest: ai.AccountList,
	}
	var err error
	ids.Root, err = s.skipchain.CreateRoster(ai.EntityList, 2, 10,
		skipchain.VerifyNone, nil)
	if err != nil {
		return nil, err
	}
	ids.Root, ids.Data, err = s.skipchain.CreateData(ids.Root, 2, 10,
		skipchain.VerifyNone, ai.AccountList)
	if err != nil {
		return nil, err
	}

	s.setIdentityStorage(IdentityID(ids.Data.Hash), ids)

	reply := &AddIdentityReply{
		Root: ids.Root,
		Data: ids.Data,
	}

	// TODO - do we really want to propagate the errors here?
	errorStr := []string{}
	for _, e := range ids.Root.EntityList.List {
		if e.ID.Equal(s.Entity().ID) {
			continue
		}
		cr, err := sda.CreateServiceMessage(ServiceName,
			&PropagateIdentity{ids})
		if err != nil {
			return nil, err
		}
		err = s.SendRaw(e, cr)
		if err != nil {
			errorStr = append(errorStr, err.Error())
		}
	}
	if len(errorStr) > 0 {
		err = errors.New(strings.Join(errorStr, "\n"))
		return nil, err
	}
	return reply, nil
}

// PropagateIdentity stores that identity on a remote node
func (s *Service) PropagateIdentity(e *network.Entity, pi *PropagateIdentity) (network.ProtocolMessage, error) {
	id := IdentityID(pi.Data.Hash)
	if s.getIdentityStorage(id) != nil {
		return nil, errors.New("That identity already exists here")
	}
	dbg.Lvl3("Storing identity in", s)
	s.setIdentityStorage(id, pi.IdentityStorage)
	return nil, nil
}

// ProposeConfig only stores the proposed configuration internally. Signatures
// come later.
func (s *Service) ProposeConfig(e *network.Entity, p *PropagateProposition) (network.ProtocolMessage, error) {
	sid := s.getIdentityStorage(p.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	roster := sid.Root.EntityList
	replies, err := manage.PropagateStartAndWait(s, roster,
		p, 1000, s.Propagate)
	if err != nil {
		return nil, err
	}
	if replies != len(roster.List) {
		dbg.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil, nil
}

// ConfigNewCheck returns an eventual config-proposition
func (s *Service) ConfigNewCheck(e *network.Entity, cnc *ConfigNewCheck) (network.ProtocolMessage, error) {
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
func (s *Service) ConfigUpdate(e *network.Entity, cu *ConfigUpdate) (network.ProtocolMessage, error) {
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
func (s *Service) VoteConfig(e *network.Entity, v *Vote) (network.ProtocolMessage, error) {
	// First verify if the signature is legitimate
	sid := s.getIdentityStorage(v.ID)
	if sid == nil {
		return nil, errors.New("Didn't find identity")
	}
	sid.Lock()
	dbg.Lvl3("Voting on", sid.Proposed.Owners)
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
	dbg.Lvl3(v.Signer, "voted", v.Signature)
	if v.Signature != nil {
		err = crypto.VerifySchnorr(network.Suite, owner.Point, hash, *v.Signature)
		if err != nil {
			return nil, errors.New("Wrong signature: " + err.Error())
		}
	}
	sid.Unlock()

	// Propagate the vote
	_, err = manage.PropagateStartAndWait(s, sid.Root.EntityList, v, 1000, s.Propagate)
	if err != nil {
		return nil, err
	}
	if len(sid.Votes) >= sid.Latest.Threshold ||
		len(sid.Votes) == len(sid.Latest.Owners) {
		// If we have enough signatures, make a new data-skipblock and
		// propagate it
		dbg.Lvl3("Having majority or all votes")

		// Making a new data-skipblock
		dbg.Lvl3("Sending data-block with", sid.Proposed.Owners)
		reply, err := s.skipchain.ProposeData(sid.Root, sid.Data, sid.Proposed)
		if err != nil {
			return nil, err
		}
		_, msg, _ := network.UnmarshalRegistered(reply.Latest.Data)
		dbg.Lvl3("SB signed is", msg.(*AccountList).Owners)
		usb := &UpdateSkipBlock{
			ID:      v.ID,
			Latest: reply.Latest,
		}
		_, err = manage.PropagateStartAndWait(s, sid.Root.EntityList, usb, 1000, s.Propagate)
		if err != nil {
			return nil, err
		}
		return sid.Data, nil
	}
	return nil, nil
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1(s.Entity(), "Identity received New Protocol event", tn.ProtocolName(), tn, conf)
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
func (s *Service) Propagate(msg network.ProtocolMessage) {
	dbg.Lvlf4("Got msg %+v %v", msg, reflect.TypeOf(msg).String())
	id := IdentityID(nil)
	switch msg.(type) {
	case *PropagateProposition:
		id = msg.(*PropagateProposition).ID
	case *Vote:
		id = msg.(*Vote).ID
	case *UpdateSkipBlock:
		id = msg.(*UpdateSkipBlock).ID
	}

	if id != nil {
		sid := s.getIdentityStorage(id)
		if sid == nil {
			dbg.Error("Didn't find entity in", s)
		}
		sid.Lock()
		defer sid.Unlock()
		switch msg.(type){
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
			if err != nil{
				dbg.Error(err)
				return
			}
			al, ok := msgLatest.(*AccountList)
			if !ok {
				dbg.Error(err)
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
func (s *Service) getIdentityStorage(id IdentityID) *IdentityStorage {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	is, ok := s.identities[string(id)]
	if !ok {
		return nil
	}
	return is
}

// setIdentityStorage saves an IdentityStorage
func (s *Service) setIdentityStorage(id IdentityID, is *IdentityStorage) {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	dbg.Lvlf3("%s %x %v", s.Context.Entity(), id[0:8], is.Latest.Owners)
	s.identities[string(id)] = is
}

func newIdentityService(c sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		identities:       make(map[string]*IdentityStorage),
		skipchain:        skipchain.NewClient(),
		path:             path,
	}
	for _, f := range []interface{}{s.ProposeConfig, s.VoteConfig,
		s.AddIdentity, s.PropagateIdentity, s.ConfigNewCheck,
		s.ConfigUpdate} {
		if err := s.RegisterMessage(f); err != nil {
			dbg.Fatal("Registration error:", err)
		}
	}
	return s
}

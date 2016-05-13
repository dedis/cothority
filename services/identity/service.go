package identity

/*
SSH-ks is a keystorage service for SSH keys. You can have a set of clients
that communicate with a cothority to keep the list of public keys updated.
A number of servers track all changes and update their .authorized_hosts
accordingly.
*/

import (
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/skipchain"
	"errors"
	"strings"
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
	Identities map[string]*IdentityStorage
	skipchain  *skipchain.Client
	path       string
}

type IdentityStorage struct {
	Latest   *AccountList
	Proposed *AccountList
	Votes    []*crypto.SchnorrSig
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

	s.Identities[string(ids.Data.Hash)] = ids

	reply := &AddIdentityReply{
		Root: ids.Root,
		Data: ids.Data,
	}

	// TODO - do we really want to propagate the errors here?
	errorStr := []string{}
	for _, e := range ids.Root.EntityList.List{
		if e.ID.Equals(s.Entity().ID){
			continue
		}
		cr, err := sda.CreateServiceMessage(ServiceName,
			&PropagateIdentity{ids})
		if err != nil{
			return nil, err
		}
		err = s.SendRaw(e, cr)
		if err != nil{
			errorStr = append(errorStr, err.Error())
		}
	}
	if len(errorStr) > 0{
		err = errors.New(strings.Join(errorStr, "\n"))
		return nil, err
	}
	return reply, nil
}

// PropagateIdentity stores that identity on a remote node
func (s *Service) PropagateIdentity(e *network.Entity, pi *PropagateIdentity)(network.ProtocolMessage, error){
	id := string(pi.Data.Hash)
	if _, exists := s.Identities[id]; exists{
		return nil, errors.New("That identity already exists here")
	}
	dbg.Print("Storing identity in", s)
	s.Identities[id] = pi.IdentityStorage
	return nil, nil
}

// ProposeConfig only stores the proposed configuration internally. Signatures
// come later.
func (s *Service) ProposeConfig(e *network.Entity, p *Proposition) (network.ProtocolMessage, error) {
	dbg.Lvlf3("%s Storing proposed config from %x", s.Context.Address(), p.AccountList)
	sid, ok := s.Identities[string(p.ID)]
	if !ok{
		return nil, errors.New("Didn't find Identity")
	}
	sid.Proposed = p.AccountList
	return nil, nil
}

func (s *Service)ConfigNewCheck(e *network.Entity, cnc *ConfigNewCheck)(network.ProtocolMessage, error){
	sid, ok := s.Identities[string(cnc.ID)]
	if !ok{
		return nil, errors.New("Didn't find Identity")
	}
	return &ConfigNewCheck{
		ID: cnc.ID,
		AccountList: sid.Proposed,
	}, nil
}

// VoteConfig takes int account a vote for the proposed config. It also verifies
// that the voter is in the latest config.
func (s *Service) VoteConfig(e *network.Entity, v *Vote) (network.ProtocolMessage, error) {
	return nil, nil
}

// NewProtocol is called by the Overlay when a new protocol request comes in.
func (s *Service) NewProtocol(*sda.TreeNodeInstance, *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}

func newIdentityService(c sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		Identities:       make(map[string]*IdentityStorage),
		skipchain:        skipchain.NewClient(),
		path:             path,
	}
	for _, f := range []interface{}{s.ProposeConfig, s.VoteConfig,
		s.AddIdentity, s.PropagateIdentity, s.ConfigNewCheck} {
		if err := s.RegisterMessage(f); err != nil {
			dbg.Fatal("Registration error:", err)
		}
	}
	return s
}

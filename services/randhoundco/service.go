package randhoundco

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/jvss"
	randProto "github.com/dedis/cothority/protocols/randhoundco"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

const ServiceName = "randhoundco"

// Service implements the randhoundco service.
type service struct {
	*sda.ServiceProcessor
	*jvssStore
	*rootStore
}

func newRandhoundcoService(c *sda.Context, path string) sda.Service {
	s := &service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		jvssStore:        newJvssStore(),
		rootStore:        newRosterStore(),
	}
	return s
}

// Setup takes a SetupReq and start the JVSS protocols accordingly.
// The service then collects all longterm secrets and returns them to
// the client. This service's conode is then considered as the leader of the subsequents
// RandhoundCo rounds and does NOT have any JVSS groups tied to it. As a
// consequence, this service's ServerIdentity must *NOT* be contained in the
// SetupReq packet.
func (s *service) Setup(si *network.ServerIdentity, setup *SetupReq) (network.Body, error) {
	// check if we are not included
	mine := s.ServerIdentity().Public
	for _, si := range setup.Entities {
		if si.Public.Equal(mine) {
			return nil, errors.New("Can't include public key of the iniator service in the Roster")
		}
	}
	// gen the groups  XXX Again randhound will do it later
	groups, roster := s.groups(setup.Entities, setup.Groups)
	// get the tree of leaders + this service as root
	tree := roster.GenerateBinaryTree()
	// lauch the setup protocol
	tn := s.NewTreeNodeInstance(tree, tree.Root, randProto.SetupProto)

	setupProto := randProto.NewSetupClient(tn, groups)
	// Get the final groups when the setup is done
	grCh := make(chan *randProto.Groups)
	setupProto.RegisterOnDone(func(g *randProto.Groups) {
		grCh <- g
	})
	response := <-grCh
	if !bytes.Equal(response.Id, groups.Id) {
		panic("Someone's trying to trick us")
	}
	s.push(response.Id, rootInfo{roster, response.Aggregate})
	return response, nil
}

func (s *service) NewRound(si *network.ServerIdentity, req NewRoundReq) (network.Body, error) {
	// get all info on the randhoundco system
	rootInfo, e := s.get(req.Id)
	if !e {
		return nil, fmt.Errorf("No randhoundco system at this ID %x", req.Id)
	}

	// create the message as the current time
	msg, err := time.Now().MarshalBinary()
	if err != nil {
		return nil, err
	}
	// create protocol and launch it
	tree := rootInfo.GenerateNaryTree(len(rootInfo.List) - 1)
	tn := s.NewTreeNodeInstance(tree, tree.Root, randProto.ProtoName)
	if tn == nil {
		return nil, err
	}
	proto, err := randProto.NewRandhoundCoNode(tn, msg, rootInfo.aggregate)
	if err != nil {
		return nil, err
	}
	sigCh := make(chan []byte)
	proto.RegisterSignatureHook(func(sig []byte) {
		sigCh <- sig
	})
	go proto.Start()

	// wait for the signature and sends it back
	sig := <-sigCh
	return &NewRoundRep{Msg: msg, Sig: sig}, nil
}

func (s *service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	switch tn.ProtocolName() {
	case randProto.SetupProto:
		setup := randProto.NewSetupLeader(tn)
		// get the JVSS protocol associated with the leader
		setup.RegisterOnJVSSLongterm(func(id []byte, jv *jvss.JVSS) { s.registerJVSS(id, jv) })
		return setup
	default:
		return nil, errors.New("Don't know about the protocol you want me to create")
	}
}

func (s *service) groups(list []*network.ServerIdentity, nbGroup int) (randProto.GroupRequests, *sda.Roster) {
	var shard []*network.ServerIdentity
	var groups []randProto.GroupRequest
	n := len(list) / nbGroup
	leaders := []*network.ServerIdentity{s.ServerIdentity()}
	var leadersIdx []int
	for i := 0; i < len(list); i++ {
		shard = append(shard, list[i])
		if len(shard) == 1 {
			leaders = append(leaders, list[i])
			leadersIdx = append(leadersIdx, len(groups))
		}
		if (i%n == n-1) && len(groups) < nbGroup-1 {
			groups = append(groups, randProto.GroupRequest{shard})
			shard = []*network.ServerIdentity{}
		}
	}
	groups = append(groups, randProto.GroupRequest{shard})
	// generate the random identifier
	// XXX This step will also be replaced by the randhound protocol's output
	// once merged.
	var id [16]byte
	n, err := rand.Read(id[:])
	if n != 16 || err != nil {
		panic("the whole system is compromised, leave the ship")
	}
	g := &randProto.GroupRequests{id, groups, leadersIdx}
	roster := sda.NewRoster(leaders)
	return groups, leaders
}

// jvssStore is used by all leader services which launch different JVSS
// protocols mapped by their randhound ID session.
type jvssStore struct {
	sync.Mutex
	instances map[string]*jvss.JVSS
}

func newJvssStore() *jvssStore {
	return &jvssStore{
		instances: make(map[string]*jvss.JVSS),
	}
}

func (j *jvssStore) registerJVSS(id []byte, jvss *jvss.JVSS) {
	j.Lock()
	defer j.Unlock()
	j.instances[string(id)] = jvss
}

func (j *jvssStore) getJVSS(id []byte) *jvss.JVSS {
	j.Lock()
	defer j.Unlock()
	return j.instances[string(id)]
}

// rootInfo contains all informations generated by the randomness provider
// nodes,i.e. the nodes who setup a randhoundco system in place.
type rootInfo struct {
	*sda.Roster
	// the aggregate is necessary to give to the randhoundco protocol when root
	// for the moment. It's something that will likely not be needed anymore
	// after certain restructuring of JVSS. See protocols/randhoundco/proto.go
	// for more info.
	aggregate abstract.Point
}

// rootStore contains all the rootInfo  generated by the service.
type rootStore struct {
	sync.Mutex
	roster map[string]rootInfo
}

func newRosterStore() *rootStore {
	return &rootStore{
		roster: make(map[string]sda.Roster),
	}
}

// push sotres the roster associated with the given id.
func (i *rootStore) push(id []byte, info rootInfo) {
	i.Lock()
	defer i.Unlock()
	i.ids[string(id)] = r
}

// remove deletes the roster associated with this id.
func (i *rootStore) remove(id []byte) {
	i.Lock()
	defer i.Unlock()
	delete(i.ids, string(id))
}

// get returns the roster associated with this id or nil if it does not exists.
func (i *rootStore) get(id []byte) (rootInfo, bool) {
	i.Lock()
	defer i.Unlock()
	r, e := i.ids[string(id)]
	return r, e
}

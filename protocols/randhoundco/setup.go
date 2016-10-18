package randhoundco

import (
	"errors"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/poly"
)

// SetupProto is the name of the setup protocol
const SetupProto = "RandhoundCoSetup"

// setupClient is the protocol ran by the client who wishes to setup the full
// randhoundco system given some JVSS groups to create.
type setupClient struct {
	*sda.TreeNodeInstance
	// id generated for this specific setup of randhoundco
	systemID []byte
	// the groups requested
	request GroupRequests
	// groups received from the children in tree that needs to be buffered
	childrenGroup []Group
	// nb of response received from children
	nbChildrenResp int
	childrenMut    sync.Mutex
	// the final grouping created
	groups Groups

	// the callback to call when the protocol is finished
	onDone func(*Groups)
}

// NewSetupClient returns a setupClient who manages the creation of all the JVSS
// groups. He is a client by definition so is not the leader of a JVSS group.
func NewSetupClient(tn *sda.TreeNodeInstance, groups GroupRequests) (*setupClient, error) {
	s := &setupClient{
		TreeNodeInstance: tn,
		request:          groups,
	}
	return s, s.RegisterHandler(s.onResponse)
}

// Start sends down the request through the tree to the leaders.
func (s *setupClient) Start() error {
	log.Lvl2(s.Name(), "Client Start()")
	return s.SendToChildren(&s.request)
}

func (s *setupClient) onResponse(wrap wrapGroups) error {
	groups := wrap.Groups
	// buffer the response
	s.childrenMut.Lock()
	defer s.childrenMut.Unlock()
	s.childrenGroup = append(s.childrenGroup, groups.Groups...)
	s.nbChildrenResp++
	if s.nbChildrenResp < len(s.Children()) {
		return nil
	}

	defer s.Done()

	// compute the aggregate
	agg := s.Suite().Point().Null()
	for _, g := range s.childrenGroup {
		agg.Add(agg, g.Longterm)
	}

	s.groups = Groups{
		Id:        s.request.Id,
		Aggregate: agg,
		Groups:    s.childrenGroup,
	}

	if s.onDone != nil {
		s.onDone(&s.groups)
	}
	return nil
}

// RegisterOnDone registers the callback function to call when all the groups
// are created,i.e. at the end of the protocol.
func (s *setupClient) RegisterOnSetupDone(fn func(*Groups)) {
	s.onDone = fn
}

// setupLeader is the protocol ran by all leaders of a JVSS groups that they
// will create.
type setupLeader struct {
	*sda.TreeNodeInstance
	// id of the system received in from the client
	id []byte
	// instance of the created JVSS protocol
	jvss *jvss.JVSS
	// roster describing the JVSS group
	roster *sda.Roster
	// groups received from the children in tree that needs to be buffered
	childrenGroup []Group
	// channel used to communicate the longterm secret of the JVSS group
	// affiliated with this leader when it's ready.
	longtermCh chan *poly.SharedSecret
	// callback called when the jvss group + longterm secret has been generated
	onJVSS func(*jvss.JVSS)
}

// NewSetupLeader  returns a setupLeader who receives a group requests, launch
// the JVSS protocol and aggregates the longterms keys of its children's group
// and its own, and pass that up to the setupClient.
func NewSetupLeader(tn *sda.TreeNodeInstance) (*setupLeader, error) {
	s := &setupLeader{
		TreeNodeInstance: tn,
		longtermCh:       make(chan *poly.SharedSecret),
	}
	return s, s.RegisterHandlers(s.onRequest, s.onResponse)
}

func (s *setupLeader) Start() error {
	panic("Don't put me in this position I don't want")
}

func (s *setupLeader) onRequest(wrap wrapGroupRequests) error {
	request := wrap.GroupRequests
	// get our group index. -1 because the client is part of the roster.
	idx := s.Index() - 1
	if idx >= len(request.Leaders) {
		return errors.New("Am i not a JVSS leader?")
	}
	grpIdx := request.Leaders[int32(idx)]
	if int(grpIdx) >= len(request.Groups) {
		return errors.New("Is there no group for me ?")
	}
	group := request.Groups[grpIdx]
	s.roster = sda.NewRoster(group.Nodes)
	s.id = request.Id
	tree := s.roster.GenerateNaryTree(len(s.roster.List) - 1)
	log.Lvl2(s.Name(), "Launching JVSS idx", idx, " for request id", request.Id)
	// launch the protocol and fetch the longterm
	go func() {
		jv, err := s.CreateProtocol("JVSSCoSi", tree)
		if err != nil {
			log.Error(err)
			return
		}
		s.jvss = jv.(*jvss.JVSS)
		if err := jv.Start(); err != nil {
			log.Error(err)
			return
		}
		s.longtermCh <- s.jvss.Longterm()
	}()

	if s.IsLeaf() {
		respGroups := Groups{
			Id:     request.Id,
			Groups: []Group{},
		}
		s.onResponse(wrapGroups{Groups: respGroups})
	}
	return s.SendToChildren(request)
}

func (s *setupLeader) onResponse(wrap wrapGroups) error {
	if s.IsRoot() {
		panic("I shouldn't be in this embarrassing position")
	}
	groups := wrap.Groups
	// buffer the response
	s.childrenGroup = append(s.childrenGroup, groups.Groups...)
	if len(s.childrenGroup) < len(s.Children()) {
		return nil
	}
	defer s.Done()

	// wait for our longterm
	log.Lvl2(s.Name(), "Is waiting on JVSS's longterm")
	long := <-s.longtermCh
	log.Print(s.Name(), "Is DONE waiting on JVSS's longterm")
	if s.onJVSS != nil {
		log.Print(s.Name(), "Calling onJVSS")
		s.onJVSS(s.jvss)
	}
	// add our group to the global list
	log.Print(s.Name(), "Adding Longterm:", long.Pub.SecretCommit())
	myGroup := Group{s.roster.List, long.Pub.SecretCommit()}
	allGroups := append(s.childrenGroup, myGroup)
	groups.Groups = allGroups
	// and pass that up
	return s.SendToParent(&groups)
}

func (s *setupLeader) RegisterOnJVSS(fn func(*jvss.JVSS)) {
	s.onJVSS = fn
}

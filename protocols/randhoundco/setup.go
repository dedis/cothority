package randhoundco

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/poly"
)

// SetupProto is the name of the setup protocol
const SetupProto = "RandhoundCoSetup"

// setupRoot is the protocol ran by the client who wishes to setup the full
// randhoundco system given some JVSS groups to create.
type setupRoot struct {
	*setupNode
	// the groups requested
	request GroupRequests
	// the final grouping created
	groups Groups

	// the callback to call when the protocol is finished
	onDone func(*Groups)
}

// setupNode is the protocol ran by all leaders of a JVSS groups that they
// will create.
type setupNode struct {
	*sda.TreeNodeInstance
	// id of the system received in from the client
	id []byte
	// instance of the created JVSS protocol
	jvss *jvss.JVSS
	// roster describing the JVSS group
	roster *sda.Roster
	// groups received from the children + our own
	aggGroups []Group
	// channel used to communicate the longterm secret of the JVSS group
	// affiliated with this leader when it's ready.
	longtermCh chan *poly.SharedSecret
	// callback called when the jvss group + longterm secret has been generated
	onJVSS func(*jvss.JVSS)
}

// NewSetupRoot returns a setupRoot who manages the creation of all the JVSS
// groups. He is a client by definition so is not the leader of a JVSS group.
func NewSetupRoot(tn *sda.TreeNodeInstance, groups GroupRequests) (*setupRoot, error) {
	p, err := NewSetupNode(tn)
	if err != nil {
		return nil, err
	}
	s := &setupRoot{
		setupNode: p,
		request:   groups,
	}
	return s, s.RegisterHandler(s.onResponse)
}

// NewSetupNode  returns a setupNode who receives a group requests, launch
// the JVSS protocol and aggregates the longterms keys of its children's group
// and its own, and pass that up to the setupRoot.
func NewSetupNode(tn *sda.TreeNodeInstance) (*setupNode, error) {
	s := &setupNode{
		TreeNodeInstance: tn,
		longtermCh:       make(chan *poly.SharedSecret),
	}
	return s, s.RegisterHandlers(s.onRequest, s.onResponse)
}

// Start sends down the request through the tree to the leaders.
func (s *setupRoot) Start() error {
	log.Lvl2(s.Name(), "Client Start()")
	return s.onRequest(wrapGroupRequests{GroupRequests: s.request})
}

// Start is not supposed to be called on a setupNode.
func (s *setupNode) Start() error {
	panic("Don't put me in this position I don't want")
}

func (s *setupNode) onRequest(wrap wrapGroupRequests) error {
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
		s.onResponse([]wrapGroups{wrapGroups{Groups: respGroups}})
	}
	return s.SendToChildren(request)
}

// onResponse call the setupNode.onResponse and then dispatch the grouping if a
// hook have been registered.
func (s *setupRoot) onResponse(wraps []wrapGroups) error {
	if err := s.setupNode.onResponse(wraps); err != nil {
		return err
	}
	aggregate := s.Suite().Point().Null()
	for _, g := range s.aggGroups {
		aggregate.Add(aggregate, g.Longterm)
	}

	s.groups = Groups{
		Id:        s.request.Id,
		Aggregate: aggregate,
		Groups:    s.aggGroups,
	}

	if s.onDone != nil {
		s.onDone(&s.groups)
	}
	return nil
}

// onResponse aggregates all groups received and create its own and sends that up
// in the tree.
func (s *setupNode) onResponse(wraps []wrapGroups) error {
	defer s.Done()
	// buffer the response
	for _, wg := range wraps {
		s.aggGroups = append(s.aggGroups, wg.Groups.Groups...)
	}

	// wait for our longterm
	log.Lvl2(s.Name(), "Is waiting on JVSS's longterm")
	long := <-s.longtermCh
	log.Print(s.Name(), "Is DONE waiting on JVSS's longterm")
	// add our group to the global list
	log.Print(s.Name(), "Adding Longterm:", long.Pub.SecretCommit())
	myGroup := Group{s.roster.List, long.Pub.SecretCommit()}
	s.aggGroups = append(s.aggGroups, myGroup)

	if s.onJVSS != nil {
		s.onJVSS(s.jvss)
	}

	if s.IsRoot() {
		return nil
	}
	// pass that up
	return s.SendToParent(&Groups{Id: s.id, Groups: s.aggGroups})
}

func (s *setupNode) RegisterOnJVSS(fn func(*jvss.JVSS)) {
	s.onJVSS = fn
}

// RegisterOnDone registers the callback function to call when all the groups
// are created,i.e. at the end of the protocol.
func (s *setupRoot) RegisterOnSetupDone(fn func(*Groups)) {
	s.onDone = fn
}

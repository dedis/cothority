package sda_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

type DummyProtocol struct {
	*sda.TreeNodeInstance
	link   chan bool
	config DummyConfig
}

type DummyConfig struct {
	A    int
	Send bool
}

type DummyMsg struct {
	A int
}

var dummyMsgType network.MessageTypeID

func init() {
	dummyMsgType = network.RegisterMessageType(DummyMsg{})
}

func NewDummyProtocol(tni *sda.TreeNodeInstance, conf DummyConfig, link chan bool) *DummyProtocol {
	return &DummyProtocol{tni, link, conf}
}

func (dm *DummyProtocol) Start() error {
	dm.link <- true
	if dm.config.Send {
		if err := dm.SendTo(dm.TreeNode(), &DummyMsg{}); err != nil {
			dbg.Error(err)
		}
		// also send to the children if any
		if !dm.IsLeaf() {
			if err := dm.SendTo(dm.Children()[0], &DummyMsg{}); err != nil {
				dbg.Error(err)
			}
		}
	}
	return nil
}

func (dm *DummyProtocol) DispatchMsg(msg *sda.Data) {
	dm.link <- true
}

// legcy reasons
func (dm *DummyProtocol) Dispatch() error {
	return nil
}

type DummyService struct {
	c        sda.Context
	path     string
	link     chan bool
	fakeTree *sda.Tree
	firstTni *sda.TreeNodeInstance
	Config   DummyConfig
}

func (ds *DummyService) ProcessClientRequest(e *network.Entity, r *sda.ClientRequest) {
	msgT, _, err := network.UnmarshalRegisteredType(r.Data, network.DefaultConstructors(network.Suite))
	if err != nil || msgT != dummyMsgType {
		ds.link <- false
		return
	}
	if ds.firstTni == nil {
		ds.firstTni = ds.c.NewTreeNodeInstance(ds.fakeTree, ds.fakeTree.Root)
	}

	dp := NewDummyProtocol(ds.firstTni, ds.Config, ds.link)

	if err := ds.c.RegisterProtocolInstance(dp); err != nil {
		ds.link <- false
		return
	}
	dp.Start()
}

func (ds *DummyService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dp := NewDummyProtocol(tn, DummyConfig{}, ds.link)
	return dp, nil
}

func (ds *DummyService) ProcessServiceMessage(e *network.Entity, s *sda.ServiceMessage) {
	id, m, err := network.UnmarshalRegisteredType(s.Data, network.DefaultConstructors(network.Suite))
	if err != nil {
		ds.link <- false
		return
	}
	if id != dummyMsgType {
		ds.link <- false
		return
	}
	dms := m.(DummyMsg)
	if dms.A != 10 {
		ds.link <- false
		return
	}
	ds.link <- true
}

func TestServiceNew(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		ds.link <- true
		return ds
	})
	go func() {
		h := sda.NewLocalHost(2000)
		h.Close()
	}()

	waitOrFatal(ds.link, t)
}

func TestServiceProcessRequest(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		return ds
	})
	host := sda.NewLocalHost(2000)
	host.Listen()
	host.StartProcessMessages()
	dbg.Lvl1("Host created and listening")
	defer host.Close()
	// Send a request to the service
	re := &sda.ClientRequest{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Data:    []byte("a"),
	}
	// fake a client
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link
	select {
	case v := <-ds.link:
		if v {
			t.Fatal("was expecting false !")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Too late")
	}
}

// Test if a request that makes the service create a new protocol works
func TestServiceRequestNewProtocol(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		return ds
	})
	host := sda.NewLocalHost(2000)
	host.Listen()
	host.StartProcessMessages()
	dbg.Lvl1("Host created and listening")
	defer host.Close()
	// create the entityList and tree
	el := sda.NewEntityList([]*network.Entity{host.Entity})
	tree := el.GenerateBinaryTree()
	// give it to the service
	ds.fakeTree = tree

	// Send a request to the service
	b, err := network.MarshalRegisteredType(&DummyMsg{10})
	dbg.ErrFatal(err)
	re := &sda.ClientRequest{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Data:    b,
	}
	// fake a client
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the
	waitOrFatalValue(ds.link, true, t)

	// Now RESEND the value so we instantiate using the SAME TREENODE
	dbg.Lvl1("Sending request AGAIN to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the
	// NOW expect false
	waitOrFatalValue(ds.link, false, t)
}

func TestServiceProtocolProcessMessage(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	var count int
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		if count == 0 {
			count++
			// the client does not need a Service
			return &DummyService{link: make(chan bool)}
		}
		ds.c = c
		ds.path = path
		ds.Config = DummyConfig{
			Send: true,
		}
		return ds
	})
	// fake a client
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()

	host := sda.NewLocalHost(2000)
	host.ListenAndBind()
	host.StartProcessMessages()
	dbg.Lvl1("Host created and listening")
	defer host.Close()
	// create the entityList and tree
	el := sda.NewEntityList([]*network.Entity{host.Entity})
	tree := el.GenerateBinaryTree()
	// give it to the service
	ds.fakeTree = tree

	// Send a request to the service
	b, err := network.MarshalRegisteredType(&DummyMsg{10})
	dbg.ErrFatal(err)
	re := &sda.ClientRequest{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Data:    b,
	}
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the protocol
	waitOrFatalValue(ds.link, true, t)

	// now wait for the same link as the protocol should have sent a message to
	// himself !
	waitOrFatalValue(ds.link, true, t)
}

// test for calling the NewProtocol method on a remote Service
func TestServiceNewProtocol(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds1 := &DummyService{
		link: make(chan bool),
		Config: DummyConfig{
			Send: true,
		},
	}
	ds2 := &DummyService{
		link: make(chan bool),
	}
	var count int
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		var localDs *DummyService
		switch count {
		case 2:
			// the client does not need a Service
			return &DummyService{link: make(chan bool)}
		case 1: // children
			localDs = ds2
		case 0: // root
			localDs = ds1
		}
		localDs.c = c
		localDs.path = path

		count++
		return localDs
	})
	host := sda.NewLocalHost(2000)
	host.ListenAndBind()
	host.StartProcessMessages()
	dbg.Lvl1("Host created and listening")
	defer host.Close()

	host2 := sda.NewLocalHost(2002)
	host2.ListenAndBind()
	host2.StartProcessMessages()
	defer host2.Close()
	// create the entityList and tree
	el := sda.NewEntityList([]*network.Entity{host.Entity, host2.Entity})
	tree := el.GenerateBinaryTree()
	// give it to the service
	ds1.fakeTree = tree

	// Send a request to the service
	b, err := network.MarshalRegisteredType(&DummyMsg{10})
	dbg.ErrFatal(err)
	re := &sda.ClientRequest{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Data:    b,
	}
	// fake a client
	client := sda.NewLocalHost(2010)
	defer client.Close()
	dbg.Lvl1("Client connecting to host")
	if _, err := client.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := client.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the protocol that Starts
	waitOrFatalValue(ds1.link, true, t)
	// now wait for the same link as the protocol should have sent a message to
	// himself !
	waitOrFatalValue(ds1.link, true, t)
	// now wait for the SECOND LINK on the SECOND HOST that the SECOND SERVICE
	// should have started (ds2) in ProcessRequest
	waitOrFatalValue(ds2.link, true, t)
}

func TestServiceProcessServiceMessage(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds1 := &DummyService{
		link: make(chan bool),
	}
	ds2 := &DummyService{
		link: make(chan bool),
	}
	var count int
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		var s *DummyService
		if count == 0 {
			s = ds1
		} else {
			s = ds2
		}
		s.c = c
		s.path = path
		return s
	})
	// create two hosts
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()
	h1 := sda.NewLocalHost(2000)
	h1.ListenAndBind()
	h1.StartProcessMessages()
	defer h1.Close()
	dbg.Lvl1("Host created and listening")

	// connect themselves
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(h1.Entity); err != nil {
		t.Fatal(err)
	}

	// create request
	m, err := sda.CreateServiceMessage("DummyService", &DummyMsg{10})
	assert.Nil(t, err)
	dbg.Lvl1("Sending request to service...")
	assert.Nil(t, h2.SendRaw(h1.Entity, m))

	// wait for the link from the Service on host 1
	waitOrFatalValue(ds1.link, true, t)
}

func TestServiceBackForthProtocol(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	defer local.CloseAll()

	// register service
	sda.RegisterNewService("BackForth", func(c sda.Context, path string) sda.Service {
		return &simpleService{
			ctx: c,
		}
	})
	// create hosts
	hosts, el, _ := local.GenTree(4, true, true, false)

	// create client
	priv, pub := sda.PrivPub()
	client := network.NewSecureTCPHost(priv, network.NewEntity(pub, ""))
	c, err := client.Open(hosts[0].Entity)
	assert.Nil(t, err)
	// create request
	r := &simpleRequest{
		Entities: el,
		Val:      10,
	}
	buff, err := network.MarshalRegisteredType(r)
	assert.Nil(t, err)

	req := &sda.ClientRequest{
		Service: sda.ServiceFactory.ServiceID("BackForth"),
		Data:    buff,
	}
	assert.Nil(t, c.Send(context.TODO(), req))

	nm, err := c.Receive(context.TODO())
	assert.Nil(t, err)

	assert.Equal(t, nm.MsgType, simpleResponseType)
	resp := nm.Msg.(simpleResponse)
	assert.Equal(t, resp.Val, 10)
}

func TestClient_Send(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	defer local.CloseAll()

	// register service
	sda.RegisterNewService("BackForth", func(c sda.Context, path string) sda.Service {
		return &simpleService{
			ctx: c,
		}
	})
	// create hosts
	hosts, el, _ := local.GenTree(4, true, true, false)
	client := sda.NewClient("BackForth")

	r := &simpleRequest{
		Entities: el,
		Val:      10,
	}
	nm, err := client.Send(hosts[0].Entity, r)
	dbg.ErrFatal(err)

	assert.Equal(t, nm.MsgType, simpleResponseType)
	resp := nm.Msg.(simpleResponse)
	assert.Equal(t, resp.Val, 10)
}

// BackForthProtocolForth & Back are messages that go down and up the tree.
// => BackForthProtocol protocol / message
type SimpleMessageForth struct {
	Val int
}

type SimpleMessageBack struct {
	Val int
}

var simpleMessageForthType = network.RegisterMessageType(SimpleMessageForth{})
var simpleMessageBackType = network.RegisterMessageType(SimpleMessageBack{})

type BackForthProtocol struct {
	*sda.TreeNodeInstance
	Val       int
	counter   int
	forthChan chan struct {
		*sda.TreeNode
		SimpleMessageForth
	}
	backChan chan struct {
		*sda.TreeNode
		SimpleMessageBack
	}
	handler func(val int)
}

func newBackForthProtocolRoot(tn *sda.TreeNodeInstance, val int, handler func(int)) (sda.ProtocolInstance, error) {
	s, err := newBackForthProtocol(tn)
	s.Val = val
	s.handler = handler
	return s, err
}

func newBackForthProtocol(tn *sda.TreeNodeInstance) (*BackForthProtocol, error) {
	s := &BackForthProtocol{
		TreeNodeInstance: tn,
	}
	err := s.RegisterChannel(&s.forthChan)
	if err != nil {
		return nil, err
	}
	err = s.RegisterChannel(&s.backChan)
	go s.dispatch()
	return s, nil
}

func (sp *BackForthProtocol) Start() error {
	// send down to children
	msg := &SimpleMessageForth{
		Val: sp.Val,
	}
	for _, ch := range sp.Children() {
		if err := sp.SendTo(ch, msg); err != nil {
			return err
		}
	}
	return nil
}

func (sp *BackForthProtocol) dispatch() {
	for {
		select {
		// dispatch the first msg down
		case m := <-sp.forthChan:
			msg := &m.SimpleMessageForth
			for _, ch := range sp.Children() {
				sp.SendTo(ch, msg)
			}
			if sp.IsLeaf() {
				if err := sp.SendTo(sp.Parent(), &SimpleMessageBack{msg.Val}); err != nil {
					dbg.Error(err)
				}
				return
			}
			// pass the message up
		case m := <-sp.backChan:
			msg := m.SimpleMessageBack
			// call the handler  if we are the root
			sp.counter++
			if sp.counter == len(sp.Children()) {
				if sp.IsRoot() {
					sp.handler(msg.Val)
				} else {
					sp.SendTo(sp.Parent(), &msg)
				}
				sp.Done()
				return
			}
		}
	}
}

// Client API request / response emulation
type simpleRequest struct {
	Entities *sda.EntityList
	Val      int
}

type simpleResponse struct {
	Val int
}

var simpleRequestType = network.RegisterMessageType(simpleRequest{})
var simpleResponseType = network.RegisterMessageType(simpleResponse{})

type simpleService struct {
	ctx sda.Context
}

func (s *simpleService) ProcessClientRequest(e *network.Entity, r *sda.ClientRequest) {
	msgT, pm, err := network.UnmarshalRegisteredType(r.Data, network.DefaultConstructors(network.Suite))
	dbg.ErrFatal(err)
	if msgT != simpleRequestType {
		return
	}
	req := pm.(simpleRequest)
	tree := req.Entities.GenerateBinaryTree()
	tni := s.ctx.NewTreeNodeInstance(tree, tree.Root)
	proto, err := newBackForthProtocolRoot(tni, req.Val, func(n int) {
		if err := s.ctx.SendRaw(e, &simpleResponse{
			Val: n,
		}); err != nil {
			dbg.Error(err)
		}
	})
	if err != nil {
		dbg.Error(err)
		return
	}
	if err := s.ctx.RegisterProtocolInstance(proto); err != nil {
		dbg.Error(err)
	}
	go proto.Start()
}

func (s *simpleService) NewProtocol(tni *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	pi, err := newBackForthProtocol(tni)
	return pi, err
}

func (s *simpleService) ProcessServiceMessage(e *network.Entity, r *sda.ServiceMessage) {
	return
}

func waitOrFatalValue(ch chan bool, v bool, t *testing.T) {
	select {
	case b := <-ch:
		if v != b {
			t.Fatal("Wrong value returned on channel")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Waited too long")
	}

}
func waitOrFatal(ch chan bool, t *testing.T) {
	select {
	case _ = <-ch:
		return
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Waited too long")
	}
}

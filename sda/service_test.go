package sda

import (
	"testing"
	"time"

	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/stretchr/testify/assert"
)

func TestServiceRegistration(t *testing.T) {
	var name = "dummy"
	RegisterNewService(name, func(c *Context, path string) Service {
		return &DummyService{}
	})

	names := ServiceFactory.RegisteredServiceNames()
	var found bool
	for _, n := range names {
		if n == name {
			found = true
		}
	}
	if !found {
		t.Fatal("Name not found !?")
	}
	ServiceFactory.Unregister(name)
	names = ServiceFactory.RegisteredServiceNames()
	for _, n := range names {
		if n == name {
			t.Fatal("Dummy should not be found!")
		}
	}
}

type DummyProtocol struct {
	*TreeNodeInstance
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

var dummyMsgType network.PacketTypeID

func init() {
	dummyMsgType = network.RegisterPacketType(DummyMsg{})
}

func NewDummyProtocol(tni *TreeNodeInstance, conf DummyConfig, link chan bool) *DummyProtocol {
	return &DummyProtocol{tni, link, conf}
}

func (dm *DummyProtocol) Start() error {
	dm.link <- true
	if dm.config.Send {
		// also send to the children if any
		if !dm.IsLeaf() {
			if err := dm.SendToChildren(&DummyMsg{}); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

func (dm *DummyProtocol) ProcessProtocolMsg(msg *ProtocolMsg) {
	dm.link <- true
}

// legcy reasons
func (dm *DummyProtocol) Dispatch() error {
	return nil
}

type DummyService struct {
	c        *Context
	path     string
	link     chan bool
	fakeTree *Tree
	firstTni *TreeNodeInstance
	Config   DummyConfig
}

func (ds *DummyService) ProcessClientRequest(si *network.ServerIdentity, r *ClientRequest) {
	msgT, _, err := network.UnmarshalRegisteredType(r.Data, network.DefaultConstructors(network.Suite))
	if err != nil || msgT != dummyMsgType {
		ds.link <- false
		return
	}
	if ds.firstTni == nil {
		ds.firstTni = ds.c.NewTreeNodeInstance(ds.fakeTree, ds.fakeTree.Root, "DummyService")
	}

	dp := NewDummyProtocol(ds.firstTni, ds.Config, ds.link)

	if err := ds.c.RegisterProtocolInstance(dp); err != nil {
		ds.link <- false
		return
	}
	dp.Start()
}

func (ds *DummyService) NewProtocol(tn *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	dp := NewDummyProtocol(tn, DummyConfig{}, ds.link)
	return dp, nil
}

func (ds *DummyService) Process(packet *network.Packet) {
	if packet.MsgType != dummyMsgType {
		ds.link <- false
		return
	}
	dms := packet.Msg.(DummyMsg)
	if dms.A != 10 {
		ds.link <- false
		return
	}
	ds.link <- true
}

func TestServiceNew(t *testing.T) {
	ds := &DummyService{
		link: make(chan bool),
	}
	RegisterNewService("DummyService", func(c *Context, path string) Service {
		ds.c = c
		ds.path = path
		ds.link <- true
		return ds
	})
	defer UnregisterService("DummyService")
	go func() {
		local := NewLocalTest()
		local.GenConodes(1)
		defer local.CloseAll()
	}()

	waitOrFatal(ds.link, t)
	log.ErrFatal(UnregisterService("DummyService"))
}

func TestServiceProcessRequest(t *testing.T) {
	ds := &DummyService{
		link: make(chan bool),
	}
	log.ErrFatal(RegisterNewService("DummyService", func(c *Context, path string) Service {
		ds.c = c
		ds.path = path
		return ds
	}))

	defer UnregisterService("DummyService")
	local := NewLocalTest()
	hs := local.GenConodes(2)
	conode := hs[0]
	log.Lvl1("Host created and listening")
	defer local.CloseAll()
	// Send a request to the service
	re := &ClientRequest{
		Service: ServiceFactory.ServiceID("DummyService"),
		Data:    []byte("a"),
	}
	// fake a client
	client := hs[1]
	log.Lvl1("Sending request to service...")
	if err := client.Send(conode.ServerIdentity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link
	if <-ds.link {
		t.Fatal("was expecting false !")
	}
	log.ErrFatal(ServiceFactory.Unregister("DummyService"))
}

// Test if a request that makes the service create a new protocol works
func TestServiceRequestNewProtocol(t *testing.T) {
	ds := &DummyService{
		link: make(chan bool),
	}
	RegisterNewService("DummyService", func(c *Context, path string) Service {
		ds.c = c
		ds.path = path
		return ds
	})

	defer UnregisterService("DummyService")
	local := NewLocalTest()
	hs := local.GenConodes(2)
	conode := hs[0]
	client := hs[1]
	defer local.CloseAll()
	// create the entityList and tree
	el := NewRoster([]*network.ServerIdentity{conode.ServerIdentity})
	tree := el.GenerateBinaryTree()
	// give it to the service
	ds.fakeTree = tree

	// Send a request to the service
	b, err := network.MarshalRegisteredType(&DummyMsg{10})
	log.ErrFatal(err)
	re := &ClientRequest{
		Service: ServiceFactory.ServiceID("DummyService"),
		Data:    b,
	}
	// fake a client
	log.Lvl1("Sending request to service...")
	if err := client.Send(conode.ServerIdentity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the
	waitOrFatalValue(ds.link, true, t)

	// Now RESEND the value so we instantiate using the SAME TREENODE
	log.Lvl1("Sending request AGAIN to service...")
	if err := client.Send(conode.ServerIdentity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the
	// NOW expect false
	waitOrFatalValue(ds.link, false, t)
	log.ErrFatal(UnregisterService("DummyService"))
}

// test for calling the NewProtocol method on a remote Service
func TestServiceNewProtocol(t *testing.T) {
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
	countMutex := sync.Mutex{}
	RegisterNewService("DummyService", func(c *Context, path string) Service {
		countMutex.Lock()
		defer countMutex.Unlock()
		log.Lvl2("Creating service", count)
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

	defer UnregisterService("DummyService")
	local := NewLocalTest()
	defer local.CloseAll()
	hs := local.GenConodes(3)
	conode1, conode2, client := hs[0], hs[1], hs[2]
	log.Lvl1("Host created and listening")

	// create the entityList and tree
	el := NewRoster([]*network.ServerIdentity{conode1.ServerIdentity, conode2.ServerIdentity})
	tree := el.GenerateBinaryTree()
	// give it to the service
	ds1.fakeTree = tree

	// Send a request to the service
	b, err := network.MarshalRegisteredType(&DummyMsg{10})
	log.ErrFatal(err)
	re := &ClientRequest{
		Service: ServiceFactory.ServiceID("DummyService"),
		Data:    b,
	}
	log.Lvl1("Sending request to service...")
	if err := client.Send(conode1.ServerIdentity, re); err != nil {
		t.Fatal(err)
	}
	log.Lvl1("Waiting for end")
	// wait for the link from the protocol that Starts
	waitOrFatalValue(ds1.link, true, t)
	// now wait for the second link on the second HOST that the second service
	// should have started (ds2) in ProcessRequest
	waitOrFatalValue(ds2.link, true, t)
	log.Lvl1("Done")
	log.ErrFatal(ServiceFactory.Unregister("DummyService"))
}

func TestServiceProcessor(t *testing.T) {
	ds1 := &DummyService{
		link: make(chan bool),
	}
	ds2 := &DummyService{
		link: make(chan bool),
	}
	var count int
	RegisterNewService("DummyService", func(c *Context, path string) Service {
		var s *DummyService
		if count == 0 {
			s = ds1
		} else {
			s = ds2
		}
		s.c = c
		s.path = path
		c.RegisterProcessor(s, dummyMsgType)
		return s
	})
	local := NewLocalTest()
	defer local.CloseAll()
	hs := local.GenConodes(2)
	conode1, conode2 := hs[0], hs[1]

	defer UnregisterService("DummyService")
	// create two conodes
	log.Lvl1("Host created and listening")
	// create request
	log.Lvl1("Sending request to service...")
	assert.Nil(t, conode2.Send(conode1.ServerIdentity, &DummyMsg{10}))

	// wait for the link from the Service on conode 1
	waitOrFatalValue(ds1.link, true, t)
	log.ErrFatal(ServiceFactory.Unregister("DummyService"))
}

type clientProc struct {
	t     *testing.T
	relay chan SimpleResponse
}

func newClientProc(t *testing.T) *clientProc {
	return &clientProc{
		relay: make(chan SimpleResponse),
	}
}

func (c *clientProc) Process(p *network.Packet) {
	if p.MsgType != SimpleResponseType {
		c.t.Fatal("Message type not SimpleResponseType")
	}
	c.relay <- p.Msg.(SimpleResponse)
}

func TestServiceBackForthProtocol(t *testing.T) {
	local := NewLocalTest()
	defer local.CloseAll()

	// register service
	RegisterNewService("BackForth", func(c *Context, path string) Service {
		return &simpleService{
			ctx: c,
		}
	})
	// create conodes
	conodes, el, _ := local.GenTree(4, false)

	// create client
	client := local.NewClient("BackForth")

	// create request
	r := &SimpleRequest{
		ServerIdentities: el,
		Val:              10,
	}
	resp, err := client.Send(conodes[0].ServerIdentity, r)
	if err != nil {
		t.Fatal(t, err)
	}

	assert.Equal(t, resp.Msg.(SimpleResponse).Val, 10)
}

func TestClient_Send(t *testing.T) {
	local := NewLocalTest()
	defer local.CloseAll()

	// register service
	RegisterNewService("BackForth", func(c *Context, path string) Service {
		return &simpleService{
			ctx: c,
		}
	})
	// create conodes
	conodes, el, _ := local.GenTree(4, false)
	client := local.NewClient("BackForth")

	r := &SimpleRequest{
		ServerIdentities: el,
		Val:              10,
	}
	nm, err := client.Send(conodes[0].ServerIdentity, r)
	log.ErrFatal(err)

	assert.Equal(t, nm.MsgType, SimpleResponseType)
	resp := nm.Msg.(SimpleResponse)
	assert.Equal(t, resp.Val, 10)
	log.ErrFatal(ServiceFactory.Unregister("BackForth"))
}

func TestClient_LocalSend(t *testing.T) {
	local := NewLocalTest()
	defer local.CloseAll()

	// register service
	RegisterNewService("BackForth", func(c *Context, path string) Service {
		return &simpleService{
			ctx: c,
		}
	})
	// create conodes
	conodes, el, _ := local.GenTree(4, false)
	client := local.NewClient("BackForth")

	r := &SimpleRequest{
		ServerIdentities: el,
		Val:              10,
	}
	nm, err := client.Send(conodes[0].ServerIdentity, r)
	log.ErrFatal(err)

	assert.Equal(t, nm.MsgType, SimpleResponseType)
	resp := nm.Msg.(SimpleResponse)
	assert.Equal(t, resp.Val, 10)
	log.ErrFatal(ServiceFactory.Unregister("BackForth"))
}

func TestClient_Parallel(t *testing.T) {
	nbrNodes := 4
	nbrParallel := 20
	local := NewLocalTest()
	defer local.CloseAll()

	// register service
	RegisterNewService("BackForth", func(c *Context, path string) Service {
		return &simpleService{
			ctx: c,
		}
	})
	// create conodes
	conodes, el, _ := local.GenTree(nbrNodes, true)

	wg := sync.WaitGroup{}
	wg.Add(nbrParallel)
	for i := 0; i < nbrParallel; i++ {
		go func(i int) {
			log.Lvl1("Starting message", i)
			r := &SimpleRequest{
				ServerIdentities: el,
				Val:              10 * i,
			}
			client := local.NewClient("BackForth")
			nm, err := client.Send(conodes[0].ServerIdentity, r)
			log.ErrFatal(err)

			assert.Equal(t, nm.MsgType, SimpleResponseType)
			resp := nm.Msg.(SimpleResponse)
			assert.Equal(t, resp.Val, 10*i)
			log.Lvl1("Done with message", i)
			wg.Done()
		}(i)
	}
	wg.Wait()
	log.ErrFatal(ServiceFactory.Unregister("BackForth"))
}

func TestServiceManager_Service(t *testing.T) {
	local := NewLocalTest()
	defer local.CloseAll()
	conodes, _, _ := local.GenTree(2, true)

	services := conodes[0].serviceManager.AvailableServices()
	assert.NotEqual(t, 0, len(services), "no services available")

	service := conodes[0].serviceManager.Service("testService")
	assert.NotNil(t, service, "Didn't find service testService")
}

// BackForthProtocolForth & Back are messages that go down and up the tree.
// => BackForthProtocol protocol / message
type SimpleMessageForth struct {
	Val int
}

type SimpleMessageBack struct {
	Val int
}

var simpleMessageForthType = network.RegisterPacketType(SimpleMessageForth{})
var simpleMessageBackType = network.RegisterPacketType(SimpleMessageBack{})

type BackForthProtocol struct {
	*TreeNodeInstance
	Val       int
	counter   int
	forthChan chan struct {
		*TreeNode
		SimpleMessageForth
	}
	backChan chan struct {
		*TreeNode
		SimpleMessageBack
	}
	handler func(val int)
}

func newBackForthProtocolRoot(tn *TreeNodeInstance, val int, handler func(int)) (ProtocolInstance, error) {
	s, err := newBackForthProtocol(tn)
	s.Val = val
	s.handler = handler
	return s, err
}

func newBackForthProtocol(tn *TreeNodeInstance) (*BackForthProtocol, error) {
	s := &BackForthProtocol{
		TreeNodeInstance: tn,
	}
	err := s.RegisterChannel(&s.forthChan)
	if err != nil {
		return nil, err
	}
	err = s.RegisterChannel(&s.backChan)
	if err != nil {
		return nil, err
	}
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
					log.Error(err)
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
type SimpleRequest struct {
	ServerIdentities *Roster
	Val              int
}

type SimpleResponse struct {
	Val int
}

var SimpleRequestType = network.RegisterPacketType(SimpleRequest{})
var SimpleResponseType = network.RegisterPacketType(SimpleResponse{})

type simpleService struct {
	ctx *Context
}

func (s *simpleService) ProcessClientRequest(si *network.ServerIdentity, r *ClientRequest) {
	msgT, pm, err := network.UnmarshalRegisteredType(r.Data, network.DefaultConstructors(network.Suite))
	log.ErrFatal(err)
	if msgT != SimpleRequestType {
		return
	}
	req := pm.(SimpleRequest)
	tree := req.ServerIdentities.GenerateBinaryTree()
	tni := s.ctx.NewTreeNodeInstance(tree, tree.Root, "BackForth")
	proto, err := newBackForthProtocolRoot(tni, req.Val, func(n int) {
		if err := s.ctx.SendRaw(si, &SimpleResponse{
			Val: n,
		}); err != nil {
			log.Print(err)
		}
	})
	if err != nil {
		log.Print(err)
		return
	}
	if err := s.ctx.RegisterProtocolInstance(proto); err != nil {
		log.Print(err)
	}
	go proto.Start()
}

func (s *simpleService) NewProtocol(tni *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	pi, err := newBackForthProtocol(tni)
	return pi, err
}

func (s *simpleService) Process(packet *network.Packet) {
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

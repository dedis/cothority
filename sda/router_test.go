package sda

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

// Returns a ServerIdentity out of the address
func NewServerIdentity(address string) *network.ServerIdentity {
	kp := config.NewKeyPair(network.Suite)
	e := network.NewServerIdentity(kp.Public, address)
	return e
}

// localRouterStore keeps tracks of all the mock routers
type localRouterStore struct {
	localRouters map[network.ServerIdentityID]*localRouter
	mut          sync.Mutex
}

// localRouters is the store that keeps tracks of all opened local routers in a
// thread safe manner
var localRouters = localRouterStore{
	localRouters: make(map[network.ServerIdentityID]*localRouter),
}

func (lrs *localRouterStore) Put(r *localRouter) {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	lrs.localRouters[r.identity.ID] = r
}

// Get returns the router associated with this ServerIdentity. It returns nil if
// there is no localRouter associated with this ServerIdentity
func (lrs *localRouterStore) Get(id *network.ServerIdentity) *localRouter {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	r, ok := lrs.localRouters[id.ID]
	if !ok {
		return nil
	}
	return r
}

func (lrs *localRouterStore) Len() int {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	return len(lrs.localRouters)
}

// localRouter is a struct that implements the Router interface locally
type localRouter struct {
	Dispatcher
	identity *network.ServerIdentity
	msgChan  chan *network.Packet
}

func NewLocalRouter(identity *network.ServerIdentity) *localRouter {
	r := &localRouter{
		Dispatcher: NewBlockingDispatcher(),
		identity:   identity,
		msgChan:    make(chan *network.Packet),
	}
	localRouters.Put(r)
	// XXX Will be replaced by Start or Listen from the Router interface
	// go r.dispatch()
	return r
}

func (m *localRouter) SendRaw(e *network.ServerIdentity, msg network.Body) error {
	r := localRouters.Get(e)
	if r == nil {
		return errors.New("No mock routers at this entity")
	}
	// simulate network marshaling / unmarshaling
	b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	t, unmarshalled, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		return err
	}
	nm := network.Packet{
		Msg:            unmarshalled,
		MsgType:        t,
		ServerIdentity: m.identity,
	}
	r.msgChan <- &nm
	return nil
}

func (m *localRouter) Listen() {
	ready := make(chan bool)
	go func() {
		ready <- true
		for msg := range m.msgChan {
			// XXX Do we need a go routine here ?
			m.Dispatch(msg)
		}
	}()
	<-ready
}

func (m *localRouter) ServerIdentity() *network.ServerIdentity {
	return m.identity
}
func (m *localRouter) Close() {
	close(m.msgChan)
}

func (m *localRouter) Tx() uint64 {
	return 0
}

func (l *localRouter) Rx() uint64 {
	return 0
}

func (l *localRouter) GetStatus() Status {
	m := make(map[string]string)
	m["localRouters"] = strconv.Itoa(localRouters.Len())
	return m
}

func (l *localRouter) Address() string {
	return l.identity.First()
}

func (l *localRouter) ListenAndBind() {
	l.Listen()
}

func TestLocalRouter(t *testing.T) {
	m1 := NewLocalRouter(NewServerIdentity("127.0.0.1:2000"))
	m1.Listen()
	defer m1.Close()
	m2 := NewLocalRouter(NewServerIdentity("127.0.0.1:4000"))
	m2.Listen()
	defer m2.Close()
	assert.NotNil(t, localRouters.Get(m1.ServerIdentity()))
	assert.NotNil(t, localRouters.Get(m2.ServerIdentity()))

	p := newSimpleProcessor()
	m2.RegisterProcessor(p, statusMsgID)

	status := &statusMessage{true, 10}
	assert.Nil(t, m1.SendRaw(m2.ServerIdentity(), status))

	select {
	case m := <-p.relay:
		if !m.Ok || m.Val != 10 {
			t.Fatal("Wrong value")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("waited too long")
	}
}

func NewMockTcpRouter(port int) *TcpRouter {
	addr := "localhost:" + strconv.Itoa(port)
	kp := config.NewKeyPair(network.Suite)
	id := network.NewServerIdentity(kp.Public, addr)
	return NewTcpRouter(id, kp.Secret)
}

func (t *TcpRouter) abortConnections() error {
	t.closeConnections()
	close(t.ProcessMessagesQuit)
	return t.host.Close()
}

func TestTcpRouterReconnection(t *testing.T) {
	log.TestOutput(true, 5)
	h1 := NewMockTcpRouter(2000)
	h2 := NewMockTcpRouter(2001)
	/* h1 := NewLocalHost(2000)*/
	/*h2 := NewLocalHost(2001)*/
	defer h1.Close()
	defer h2.Close()

	h1.ListenAndBind()
	h1.StartProcessMessages()
	h2.ListenAndBind()
	h2.StartProcessMessages()

	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))
	log.Lvl1("Sending h2->h1")
	log.ErrFatal(sendrcv_proc(h2, h1))
	log.Lvl1("Closing h1")
	h1.CloseConnections()

	log.Lvl1("Listening again on h1")
	h1.ListenAndBind()

	log.Lvl1("Sending h2->h1")
	log.ErrFatal(sendrcv_proc(h2, h1))
	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))

	log.Lvl1("Shutting down listener of h2")

	// closing h2, but simulate *hard* failure, without sending a FIN packet
	c2 := h1.Connection(h2.serverIdentity)
	// making h2 fails
	h2.AbortConnections()
	log.Lvl1("asking h2 to listen again")
	// making h2 backup again
	h2.ListenAndBind()
	h2.StartProcessMessages()
	// and re-registering the connection to h2 from h1
	h1.RegisterConnection(h2.serverIdentity, c2)

	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))
}

func sendrcv_proc(from, to *TcpRouter) error {
	sp := newSimpleProcessor()
	// new processing
	to.RegisterProcessor(sp, statusMsgID)
	if err := from.SendRaw(to.serverIdentity, &statusMessage{true, 10}); err != nil {
		return err
	}
	var err error
	select {
	case <-sp.relay:
		err = nil
	case <-time.After(1 * time.Second):
		err = errors.New("timeout")
	}
	// delete the processing
	to.RegisterProcessor(nil, statusMsgID)
	return err
}

type statusMessage struct {
	Ok  bool
	Val int
}

var statusMsgID network.MessageTypeID = network.RegisterMessageType(statusMessage{})

type simpleProcessor struct {
	relay chan statusMessage
}

func newSimpleProcessor() *simpleProcessor {
	return &simpleProcessor{
		relay: make(chan statusMessage),
	}
}
func (sp *simpleProcessor) Process(msg *network.Packet) {
	if msg.MsgType != statusMsgID {

		sp.relay <- statusMessage{false, 0}
	}
	sm := msg.Msg.(statusMessage)

	sp.relay <- sm
}

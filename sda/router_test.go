package sda

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

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

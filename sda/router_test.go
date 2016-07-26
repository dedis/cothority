package sda

import (
	"errors"
	"strconv"
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
	h1.closeConnections()

	log.Lvl1("Listening again on h1")
	h1.ListenAndBind()

	log.Lvl1("Sending h2->h1")
	log.ErrFatal(sendrcv_proc(h2, h1))
	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))

	log.Lvl1("Shutting down listener of h2")

	// closing h2, but simulate *hard* failure, without sending a FIN packet
	c2 := h1.connection(h2.serverIdentity)
	// making h2 fails
	h2.AbortConnections()
	log.Lvl1("asking h2 to listen again")
	// making h2 backup again
	h2.ListenAndBind()
	h2.StartProcessMessages()
	// and re-registering the connection to h2 from h1
	h1.registerConnection(c2)

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

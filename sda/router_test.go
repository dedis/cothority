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

// Test setting up of Host
func TestTcpRouterNew(t *testing.T) {
	h1 := NewMockTcpRouter(2000)
	if h1 == nil {
		t.Fatal("Couldn't setup a Host")
	}
	err := h1.Close()
	if err != nil {
		t.Fatal("Couldn't close", err)
	}
}

// Test closing and opening of Host on same address
func TestTcpRouterClose(t *testing.T) {
	h1 := NewMockTcpRouter(2000)
	h2 := NewMockTcpRouter(2001)
	h1.ListenAndBind()
	_, err := h2.Connect(h1.serverIdentity)
	if err != nil {
		t.Fatal("Couldn't Connect()", err)
	}
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	log.Lvl3("Finished first connection, starting 2nd")
	h3 := NewLocalHost(2002)
	h3.ListenAndBind()
	c, err := h2.Connect(h3.ServerIdentity)
	if err != nil {
		t.Fatal(h2, "Couldn Connect() to", h3)
	}
	log.Lvl3("Closing h3")
	err = h3.Close()
	if err != nil {
		// try closing the underlying connection manually and fail
		c.Close()
		t.Fatal("Couldn't Close()", h3)
	}
}

func TestTcpRouterClose2(t *testing.T) {
	local := NewLocalTest()
	defer local.CloseAll()

	_, _, tree := local.GenTree(2, false, true, true)
	log.Lvl3(tree.Dump())
	time.Sleep(time.Millisecond * 100)
	log.Lvl3("Done")
}

func TestTcpRouterReconnection(t *testing.T) {
	h1 := NewMockTcpRouter(2000)
	h2 := NewMockTcpRouter(2001)
	/* h1 := NewLocalHost(2000)*/
	/*h2 := NewLocalHost(2001)*/
	defer h1.Close()
	defer h2.Close()

	h1.ListenAndBind()
	h2.ListenAndBind()

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
	// and re-registering the connection to h2 from h1
	h1.registerConnection(c2)

	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))
}

// Test the automatic connection upon request
func TestTcpRouterAutoConnection(t *testing.T) {
	h1 := NewMockTcpRouter(2000)
	h2 := NewMockTcpRouter(2001)
	h2.ListenAndBind()
	proc := newSimpleMessageProc(t)
	h2.RegisterProcessor(proc, SimpleMessageType)

	defer h1.Close()
	defer h2.Close()

	err := h1.SendRaw(h2.serverIdentity, &SimpleMessage{12})
	if err != nil {
		t.Fatal("Couldn't send message:", err)
	}

	// Receive the message
	msg := <-proc.relay
	if msg.I != 12 {
		t.Fatal("Simple message got distorted")
	}
}

// Test connection of multiple Hosts and sending messages back and forth
// also tests for the counterIO interface that it works well
func TestTcpRouterMessaging(t *testing.T) {
	h1, h2 := TwoTcpHosts()
	defer h1.Close()
	defer h2.Close()

	bw1 := h1.Tx()
	br2 := h2.Rx()
	proc := &simpleMessageProc{t, make(chan SimpleMessage)}
	h1.RegisterProcessor(proc, SimpleMessageType)
	h2.RegisterProcessor(proc, SimpleMessageType)

	msgSimple := &SimpleMessage{3}
	err := h1.SendRaw(h2.ServerIdentity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send from h2 -> h1:", err)
	}
	decoded := <-proc.relay
	if decoded.I != 3 {
		t.Fatal("Received message from h2 -> h1 is wrong")
	}

	written := h1.Tx() - bw1
	read := h2.Rx() - br2
	if written == 0 || read == 0 || written != read {
		t.Logf("Before => bw1 = %d vs br2 = %d", bw1, br2)
		t.Logf("Tx = %d, Rx = %d", written, read)
		t.Logf("h1.Tx() %d vs h2.Rx() %d", h1.Tx(), h2.Rx())
		t.Fatal("Something is wrong with Host.CounterIO")
	}

}

// Test sending data back and forth using the sendSDAData
func TestTcpRouterSendMsgDuplex(t *testing.T) {
	h1, h2 := TwoTcpHosts()
	proc := &simpleMessageProc{t, make(chan SimpleMessage)}
	h1.RegisterProcessor(proc, SimpleMessageType)
	h2.RegisterProcessor(proc, SimpleMessageType)

	msgSimple := &SimpleMessage{5}
	err := h1.SendRaw(h2.ServerIdentity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h1 to h2", err)
	}
	msg := <-proc.relay
	log.Lvl2("Received msg h1 -> h2", msg)

	err = h2.SendRaw(h1.ServerIdentity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h2 to h1", err)
	}
	msg = <-proc.relay
	log.Lvl2("Received msg h2 -> h1", msg)

	h1.Close()
	h2.Close()
}

func TwoTcpHosts() (*Host, *Host) {
	h1 := NewLocalHost(2000)
	h2 := NewLocalHost(2001)
	h1.ListenAndBind()
	h2.ListenAndBind()

	return h1, h2
}

func TwoTestHosts() (*Host, *Host) {
	h1 := NewTestHost(2000)
	h2 := NewTestHost(2001)
	h1.Listen()
	h2.Listen()
	return h1, h2
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

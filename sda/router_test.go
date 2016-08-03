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
	go m1.Run()
	defer m1.Close()
	m2 := NewLocalRouter(NewServerIdentity("127.0.0.1:4000"))
	go m2.Run()
	defer m2.Close()
	assert.NotNil(t, localRelays.Get(m1.identity))
	assert.NotNil(t, localRelays.Get(m2.identity))

	p := newSimpleProcessor()
	m2.RegisterProcessor(p, statusMsgID)

	status := &statusMessage{true, 10}
	assert.Nil(t, m1.Send(m2.identity, status))

	select {
	case m := <-p.relay:
		if !m.Ok || m.Val != 10 {
			t.Fatal("Wrong value")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("waited too long")
	}
}

func NewMockTcpRouter(port int) *TCPRouter {
	addr := "localhost:" + strconv.Itoa(port)
	kp := config.NewKeyPair(network.Suite)
	id := network.NewServerIdentity(kp.Public, addr)
	return NewTCPRouter(id, kp.Secret)
}

func (t *TCPRouter) abortConnections() error {
	t.closeConnections()
	close(t.quitProcessMsg)
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
	go h1.Run()
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
	h3 := NewTCPHost(2002)
	go h3.Run()
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

// Test if TCPRouter fits the interface such as calling Run(), then Close(),
// should return
func TestTcpRouterRunClose(t *testing.T) {
	h := NewMockTcpRouter(2000)
	var stop = make(chan bool)
	go func() {
		stop <- true
		h.Run()
		stop <- true
	}()
	<-stop
	// Time needed so the listener is up. Equivalent to "connecting ourself" as
	// we had before.
	time.Sleep(500 * time.Millisecond)
	h.Close()
	select {
	case <-stop:
		return
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TcpHost should have returned from Run() by now")
	}
}

func TestTcpRouterReconnection(t *testing.T) {
	h1 := NewMockTcpRouter(2000)
	h2 := NewMockTcpRouter(2001)
	/* h1 := NewLocalHost(2000)*/
	/*h2 := NewLocalHost(2001)*/
	defer h1.Close()
	defer h2.Close()

	go h1.Run()
	go h2.Run()

	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))
	log.Lvl1("Sending h2->h1")
	log.ErrFatal(sendrcv_proc(h2, h1))
	log.Lvl1("Closing h1")
	h1.closeConnections()

	log.Lvl1("Listening again on h1")
	go h1.Run()

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
	go h2.Run()
	// and re-registering the connection to h2 from h1
	h1.registerConnection(c2)

	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))
}

// Test the automatic connection upon request
func TestTcpRouterAutoConnection(t *testing.T) {
	h1 := NewMockTcpRouter(2000)
	h2 := NewMockTcpRouter(2001)
	go h2.Run()
	proc := newSimpleMessageProc(t)
	h2.RegisterProcessor(proc, SimpleMessageType)

	defer h1.Close()
	defer h2.Close()

	err := h1.Send(h2.serverIdentity, &SimpleMessage{12})
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
	err := h1.Send(h2.ServerIdentity, msgSimple)
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
	err := h1.Send(h2.ServerIdentity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h1 to h2", err)
	}
	msg := <-proc.relay
	log.Lvl2("Received msg h1 -> h2", msg)

	err = h2.Send(h1.ServerIdentity, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h2 to h1", err)
	}
	msg = <-proc.relay
	log.Lvl2("Received msg h2 -> h1", msg)

	h1.Close()
	h2.Close()
}

func TwoTcpHosts() (*Host, *Host) {
	h1 := NewTCPHost(2000)
	h2 := NewTCPHost(2001)
	go h1.Run()
	go h2.Run()

	return h1, h2
}

func TwoTestHosts() (*Host, *Host) {
	h1 := NewLocalHost(2000)
	h2 := NewLocalHost(2001)
	go h1.Run()
	go h2.Run()
	return h1, h2
}

func sendrcv_proc(from, to *TCPRouter) error {
	sp := newSimpleProcessor()
	// new processing
	to.RegisterProcessor(sp, statusMsgID)
	if err := from.Send(to.serverIdentity, &statusMessage{true, 10}); err != nil {
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

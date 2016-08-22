package network

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

// Test setting up of Host
func TestTCPHostNew(t *testing.T) {
	h1 := NewTestTCPHost(2000)
	if h1 == nil {
		t.Fatal("Couldn't setup a Host")
	}
	err := h1.Stop()
	if err != nil {
		t.Fatal("Couldn't close", err)
	}
}

// Test closing and opening of Host on same address
func TestTCPHostClose(t *testing.T) {
	h1 := NewTestTCPHost(2001)
	h2 := NewTestTCPHost(2002)
	go h1.Start()
	_, err := h2.newConn(h1.id)
	if err != nil {
		t.Fatal("Couldn't Connect()", err)
	}
	err = h1.Stop()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Stop()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	log.Lvl3("Finished first connection, starting 2nd")
	h3 := NewTestTCPHost(2003)
	go h3.Start()
	c, err := h2.newConn(h3.id)
	if err != nil {
		t.Fatal(h2, "Couldn Connect() to", h3)
	}
	log.Lvl3("Closing h3")
	err = h3.Stop()
	if err != nil {
		// try closing the underlying connection manually and fail
		c.Close()
		t.Fatal("Couldn't Stop()", h3)
	}
}

// Test if TCPRouter fits the interface such as calling Run(), then Stop(),
// should return
func TestTcpRouterRunClose(t *testing.T) {
	h := NewTestTCPHost(2004)
	var stop = make(chan bool)
	go func() {
		stop <- true
		h.Start()
		stop <- true
	}()
	<-stop
	// Time needed so the listener is up. Equivalent to "connecting ourself" as
	// we had before.
	time.Sleep(500 * time.Millisecond)
	h.Stop()
	select {
	case <-stop:
		return
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TcpHost should have returned from Run() by now")
	}
}

// Test the automatic connection upon request
func TestTCPHostAutoConnection(t *testing.T) {
	h1 := NewTestTCPHost(2007)
	h2 := NewTestTCPHost(2008)
	go h2.Start()

	proc := newSimpleMessageProc(t)
	h2.RegisterProcessor(proc, SimpleMessageType)
	defer func() {
		h1.Stop()
		h2.Stop()
		time.Sleep(250 * time.Millisecond)
	}()

	err := h1.Send(h2.id, &SimpleMessage{12})
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
func TestTCPHostMessaging(t *testing.T) {
	h1 := NewTestTCPHost(2009)
	h2 := NewTestTCPHost(2010)
	go h1.Start()
	go h2.Start()

	defer func() {
		h1.Stop()
		h2.Stop()
		time.Sleep(250 * time.Millisecond)
	}()

	bw1 := h1.Tx()
	br2 := h2.Rx()
	proc := &simpleMessageProc{t, make(chan SimpleMessage)}
	h1.RegisterProcessor(proc, SimpleMessageType)
	h2.RegisterProcessor(proc, SimpleMessageType)

	msgSimple := &SimpleMessage{3}
	err := h1.Send(h2.id, msgSimple)
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
func TestTCPHostSendMsgDuplex(t *testing.T) {
	h1 := NewTestTCPHost(2011)
	h2 := NewTestTCPHost(2012)
	go h1.Start()
	go h2.Start()

	defer func() {
		h1.Stop()
		h2.Stop()
		time.Sleep(250 * time.Millisecond)
	}()

	proc := &simpleMessageProc{t, make(chan SimpleMessage)}
	h1.RegisterProcessor(proc, SimpleMessageType)
	h2.RegisterProcessor(proc, SimpleMessageType)

	msgSimple := &SimpleMessage{5}
	err := h1.Send(h2.id, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h1 to h2", err)
	}
	msg := <-proc.relay
	log.Lvl2("Received msg h1 -> h2", msg)

	err = h2.Send(h1.id, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h2 to h1", err)
	}
	msg = <-proc.relay
	log.Lvl2("Received msg h2 -> h1", msg)
}

func TestTCPHostReconnection(t *testing.T) {
	h1 := NewTestTCPHost(2005)
	h2 := NewTestTCPHost(2006)
	defer func() {
		h1.Stop()
		h2.Stop()
		// Let some time to tcp
		time.Sleep(250 * time.Millisecond)
	}()

	go h1.Start()
	go h2.Start()

	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))
	log.Lvl1("Sending h2->h1")
	log.ErrFatal(sendrcv_proc(h2, h1))
	log.Lvl1("Closing h1")
	log.ErrFatal(h1.Stop())

	//h1 = NewTestTCPHost(2005)

	log.Lvl1("Listening again on h1")
	go h1.Start()
	time.Sleep(200 * time.Millisecond)
	log.Lvl1("Sending h2->h1")
	log.ErrFatal(sendrcv_proc(h2, h1))
	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))

	log.Lvl1("Shutting down listener of h2")

	// closing h2, but simulate *hard* failure, without sending a FIN packet
	// XXX Actually it DOES send a FIN packet: using tcphost.Close(), it closes
	// the listener AND all the connections (calling golang tcp connection
	// Close() which I'm pretty sure will send a FIN packet)
	// This test is ambiguous as it does not really simulate a network hardware
	// failure of a node, but merely a host which does weird abort
	// connections...
	// One idea if we really want to simulate that is calling tcphost.Close()
	// and at the same time, at the IP level, blocking all FIN packet.
	// Then start a new host with the same entity etc..
	// See also https://github.com/tylertreat/comcast

	/*c2 := h1.connection(h2.serverIdentity)*/
	//// making h2 fails
	//h2.AbortConnections()
	//log.Lvl1("asking h2 to listen again")
	//// making h2 backup again
	//go h2.listen()
	//// and re-registering the connection to h2 from h1
	//h1.registerConnection(c2)

	//log.Lvl1("Sending h1->h2")
	/*log.ErrFatal(sendrcv_proc(h1, h2))*/
}

// Testing exchange of entity
func TestTCPHostExchange(t *testing.T) {

	entity1 := NewTestServerIdentity("tcp://localhost:2000")
	entity2 := NewTestServerIdentity("tcp://localhost:2001")

	host1 := NewTCPHost(entity1)
	host2 := NewTCPHost(entity2)

	done := make(chan bool)
	go func() {
		done <- true
		host1.Start()
		done <- true
	}()
	<-done
	// try correctly
	c, err := NewTCPConn(entity1.Address.NetworkAddress())
	if err != nil {
		t.Fatal("Couldn't connect to host1:", err)
	}
	if err := host2.negotiateOpen(c, entity1); err != nil {
		t.Fatal("Wrong negotiation")
	}

	// try giving wrong id
	c, err = NewTCPConn(entity1.Address.NetworkAddress())
	if err != nil {
		t.Fatal("Couldn't connect to host1:", err)
	}
	if err := host2.negotiateOpen(c, entity2); err == nil {
		t.Fatal("negotiation should have aborted")
	}

	log.Lvl4("Closing connections")
	if err := host1.Stop(); err != nil {
		t.Fatal("Couldn't close host", host1)
	}
	if err := host2.Stop(); err != nil {
		t.Fatal("Couldn't close host", host2)
	}
	<-done
}

func TestChanHost(t *testing.T) {
	m1 := NewLocalHost(NewTestServerIdentity("127.0.0.1:2000"))
	go m1.Start()
	defer m1.Stop()
	m2 := NewLocalHost(NewTestServerIdentity("127.0.0.1:4000"))
	go m2.Start()
	defer m2.Stop()
	assert.NotNil(t, chanHosts.Get(m1.identity))
	assert.NotNil(t, chanHosts.Get(m2.identity))

	p := newSimpleProcessor()
	m2.RegisterProcessor(p, statusMsgID)

	status := &statusMessage{true, 10}
	assert.Nil(t, m1.Send(m2.identity, status))

	select {
	case m := <-p.relay:
		if !m.Ok || m.Val != 10 {
			t.Fatal("Wrong value")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("waited too long")
	}
}

func init() {
	SimpleMessageType = RegisterMessageType(SimpleMessage{})
}

func NewTestTCPHost(port int) *TCPHost {
	addr := "tcp://localhost:" + strconv.Itoa(port)
	return NewTCPHost(NewTestServerIdentity(addr))
}

// Returns a ServerIdentity out of the address
func NewTestServerIdentity(address string) *ServerIdentity {
	kp := config.NewKeyPair(Suite)
	e := NewServerIdentity(kp.Public, Address(address))
	return e
}

// SimpleMessage is just used to transfer one integer
type SimpleMessage struct {
	I int
}

var SimpleMessageType MessageTypeID

type simpleMessageProc struct {
	t     *testing.T
	relay chan SimpleMessage
}

func newSimpleMessageProc(t *testing.T) *simpleMessageProc {
	return &simpleMessageProc{
		t:     t,
		relay: make(chan SimpleMessage),
	}
}

func (smp *simpleMessageProc) Process(p *Packet) {
	if p.MsgType != SimpleMessageType {
		smp.t.Fatal("Wrong message")
	}
	sm := p.Msg.(SimpleMessage)
	smp.relay <- sm
}

type statusMessage struct {
	Ok  bool
	Val int
}

var statusMsgID MessageTypeID = RegisterMessageType(statusMessage{})

type simpleProcessor struct {
	relay chan statusMessage
}

func newSimpleProcessor() *simpleProcessor {
	return &simpleProcessor{
		relay: make(chan statusMessage),
	}
}
func (sp *simpleProcessor) Process(msg *Packet) {
	if msg.MsgType != statusMsgID {

		sp.relay <- statusMessage{false, 0}
	}
	sm := msg.Msg.(statusMessage)

	sp.relay <- sm
}

func sendrcv_proc(from, to *TCPHost) error {
	sp := newSimpleProcessor()
	// new processing
	to.RegisterProcessor(sp, statusMsgID)
	if err := from.Send(to.id, &statusMessage{true, 10}); err != nil {
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

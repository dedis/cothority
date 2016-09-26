package network

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

type BigMsg struct {
	Array []byte
}

// Test the receiving part of a message for tcp connections if the response is
// buffered correctly.
func TestTCPConnReceiveRaw(t *testing.T) {
	addr := make(chan string)
	done := make(chan bool)
	check := make(chan bool)

	checking := func() bool {
		select {
		case <-check:
			return false
		case <-time.After(20 * time.Millisecond):
			return true
		}
	}
	// prepare the msg
	msg := &BigMsg{Array: make([]byte, 7893)}
	_ = RegisterPacketType(BigMsg{})
	buff, err := MarshalRegisteredType(msg)
	assert.Nil(t, err)

	fn := func(c net.Conn) {
		checking = func() bool {
			select {
			case <-check:
				return false
			case <-time.After(20 * time.Millisecond):
				return true
			}
		}
		// different slices of bytes
		maxChunk := 1400
		slices := make([][]byte, 0)
		currentChunk := 0
		for currentChunk+maxChunk < len(buff) {
			slices = append(slices, buff[currentChunk:currentChunk+maxChunk])
			currentChunk += maxChunk
		}
		slices = append(slices, buff[currentChunk:])
		// send the size first
		binary.Write(c, globalOrder, Size(len(buff)))
		// then send pieces and check if the other side already returned or not
		for i, slice := range slices[:len(slices)-1] {
			t.Logf("Will write slice %d/%d...", i, len(slices))
			if n, err := c.Write(slice); err != nil || n != len(slice) {
				t.Fatal("Whut?")
			}
			t.Logf(" OK\n")
			if !checking() {
				t.Fatal("Already returned even if not finished")
			}
			time.Sleep(5 * time.Millisecond)
		}
		// the last one should make the other end return
		t.Logf("Will write last piece...")
		if n, err := c.Write(slices[len(slices)-1]); n != len(slices[len(slices)-1]) || err != nil {
			t.Fatal("could not send the last piece")
		}
		t.Logf(" OK\n")
		check <- true
	}

	go func() {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		assert.Nil(t, err)
		addr <- ln.Addr().String()
		c, err := ln.Accept()
		assert.Nil(t, err)
		// do the thing
		fn(c)
		<-done
		assert.Nil(t, ln.Close())
		done <- true
	}()

	// get addr
	listeningAddr := <-addr
	c, err := NewTCPConn(NewTCPAddress(listeningAddr))
	assert.Nil(t, err)

	buffRaw, err := c.receiveRaw()
	checking()
	if !bytes.Equal(buff, buffRaw) {
		t.Fatal("Bytes are not the same ")
	} else if err != nil {
		t.Error(err)
	}

	assert.Nil(t, c.Close())
	// tell the listener to close
	done <- true
	// wait until it is closed
	<-done

}

// test the creation of a new conn by opening a golang
// listener and making a TCPConn connect to it,then close it.
func TestTCPConn(t *testing.T) {
	addr := make(chan string)
	done := make(chan bool)

	_, err := NewTCPConn(NewTCPAddress("127.0.0.1:7878"))
	if err == nil {
		t.Fatal("Should not be able to connect here")
	}
	go func() {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		assert.Nil(t, err)
		addr <- ln.Addr().String()
		_, err = ln.Accept()
		assert.Nil(t, err)
		// wait until it can be closed
		<-done
		assert.Nil(t, ln.Close())
		done <- true
	}()

	// get addr
	listeningAddr := <-addr
	c, err := NewTCPConn(NewTCPAddress(listeningAddr))
	assert.Nil(t, err)
	assert.Equal(t, c.Local().NetworkAddress(), c.conn.LocalAddr().String())
	assert.Equal(t, c.Type(), PlainTCP)
	assert.Nil(t, c.Close())
	// tell the listener to close
	done <- true
	// wait until it is closed
	<-done
}

func TestTCPConnWithListener(t *testing.T) {
	addr := NewTCPAddress("127.0.0.1:5678")
	ln, err := NewTCPListener(addr)
	if err != nil {
		t.Fatal("error setup listener", err)
	}
	ready := make(chan bool)
	stop := make(chan bool)
	connStat := make(chan uint64)

	connFn := func(c Conn) {
		connStat <- c.Rx()
		c.Receive()
		connStat <- c.Rx()
	}
	go func() {
		ready <- true
		err := ln.Listen(connFn)
		assert.Nil(t, err, "Listener stop incorrectly")
		stop <- true
	}()

	<-ready
	c, err := NewTCPConn(addr)
	assert.Nil(t, err, "Could not open connection")
	// Test bandwitdth measurements also
	rx1 := <-connStat
	tx1 := c.Tx()
	assert.Nil(t, c.Send(&SimpleMessage{3}))
	tx2 := c.Tx()
	rx2 := <-connStat

	if (tx2 - tx1) != (rx2 - rx1) {
		t.Errorf("Connections did see same bytes? %d tx vs %d rx", (tx2 - tx1), (rx2 - rx1))
	}

	assert.Nil(t, ln.Stop(), "Error stopping listener")
	select {
	case <-stop:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Could not stop listener")

	}
}

// will create a TCPListener & open a golang net.TCPConn to it
func TestTCPListener(t *testing.T) {
	addr := NewTCPAddress("127.0.0.1:5678")
	ln, err := NewTCPListener(addr)
	if err != nil {
		t.Fatal("Error setup listener:", err)
	}
	ready := make(chan bool)
	stop := make(chan bool)
	connReceived := make(chan bool)

	connFn := func(c Conn) {
		connReceived <- true
		c.Close()
	}
	go func() {
		ready <- true
		err := ln.Listen(connFn)
		assert.Nil(t, err, "Listener stop incorrectly")
		stop <- true
	}()

	<-ready
	_, err = net.Dial("tcp", addr.NetworkAddress())
	assert.Nil(t, err, "Could not open connection")
	<-connReceived
	assert.Nil(t, ln.Stop(), "Error stopping listener")
	select {
	case <-stop:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Could not stop listener")
	}

	assert.Nil(t, ln.listen(nil))
}

func TestTCPRouter(t *testing.T) {
	wrongAddr := &ServerIdentity{Address: NewLocalAddress("127.0.0.1:2000")}
	_, err := NewTCPRouter(wrongAddr)
	if err == nil {
		t.Fatal("Should not setup Router with local address")
	}
	addr := &ServerIdentity{Address: NewTCPAddress("127.0.0.1:2000")}
	h1, err := NewTCPRouter(addr)
	if err != nil {
		t.Fatal("Could not setup host")
	}
	defer h1.Stop()
	_, err = NewTCPRouter(addr)
	if err == nil {
		t.Fatal("Should not succeed with same port")
	}
}

// Test closing and opening of Host on same address
func TestTCPHostClose(t *testing.T) {
	h1, err := NewTestTCPHost(2001)
	if err != nil {
		t.Fatal("Error setup TestTCPHost")
	}
	h2, err2 := NewTestTCPHost(2002)
	if err2 != nil {
		t.Fatal("Error setup TestTCPHost2")
	}
	go h1.Listen(acceptAndClose)
	if _, err := h2.Connect(NewLocalAddress("127.0.0.1:7878")); err == nil {
		t.Fatal("Should not connect to dummy address or different type")
	}
	_, err = h2.Connect(h1.addr)
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
	h3, err3 := NewTestTCPHost(2003)
	if err3 != nil {
		t.Fatal("Could not setup host", err)
	}
	go h3.Listen(acceptAndClose)
	_, err = h2.Connect(h3.addr)
	if err != nil {
		t.Fatal(h2, "Couldn Connect() to", h3)
	}
	log.Lvl3("Closing h3")
	err = h3.Stop()
	if err != nil {
		// try closing the underlying connection manually and fail
		t.Fatal("Couldn't Stop()", h3)
	}
}

type dummyErr struct {
	timeout   bool
	temporary bool
}

func (d *dummyErr) Timeout() bool {
	return d.timeout
}

func (d *dummyErr) Temporary() bool {
	return d.temporary
}

func (d *dummyErr) Error() string {
	return "dummy error"
}

func TestHandleError(t *testing.T) {
	assert.Equal(t, ErrClosed, handleError(errors.New("use of closed")))
	assert.Equal(t, ErrCanceled, handleError(errors.New("canceled")))
	assert.Equal(t, ErrEOF, handleError(errors.New("EOF")))

	assert.Equal(t, ErrUnknown, handleError(errors.New("Random error!")))

	de := dummyErr{true, true}
	assert.Equal(t, ErrTemp, handleError(&de))
	de.temporary = false
	assert.Equal(t, ErrTimeout, handleError(&de))
	de.timeout = false
	assert.Equal(t, ErrUnknown, handleError(&de))
}

/*func TestTCPHostReconnection(t *testing.T) {*/
//h1 := NewTestTCPHost(2005)
//h2 := NewTestTCPHost(2006)
//defer func() {
//h1.Stop()
//h2.Stop()
//// Let some time to tcp
//time.Sleep(250 * time.Millisecond)
//}()

//go h1.Start()
//go h2.Start()

//log.Lvl1("Sending h1->h2")
//log.ErrFatal(sendrcv_proc(h1, h2))
//log.Lvl1("Sending h2->h1")
//log.ErrFatal(sendrcv_proc(h2, h1))
//log.Lvl1("Closing h1")
//log.ErrFatal(h1.Stop())

////h1 = NewTestTCPHost(2005)

//log.Lvl1("Listening again on h1")
//go h1.Start()
//time.Sleep(200 * time.Millisecond)
//log.Lvl1("Sending h2->h1")
//log.ErrFatal(sendrcv_proc(h2, h1))
//log.Lvl1("Sending h1->h2")
//log.ErrFatal(sendrcv_proc(h1, h2))

//log.Lvl1("Shutting down listener of h2")

//// closing h2, but simulate *hard* failure, without sending a FIN packet
//// XXX Actually it DOES send a FIN packet: using tcphost.Close(), it closes
//// the listener AND all the connections (calling golang tcp connection
//// Close() which I'm pretty sure will send a FIN packet)
//// This test is ambiguous as it does not really simulate a network hardware
//// failure of a node, but merely a host which does weird abort
//// connections...
//// One idea if we really want to simulate that is calling tcphost.Close()
//// and at the same time, at the IP level, blocking all FIN packet.
//// Then start a new host with the same entity etc..
//// See also https://github.com/tylertreat/comcast

//[>c2 := h1.connection(h2.serverIdentity)<]
////// making h2 fails
////h2.AbortConnections()
////log.Lvl1("asking h2 to listen again")
////// making h2 backup again
////go h2.listen()
////// and re-registering the connection to h2 from h1
////h1.registerConnection(c2)

////log.Lvl1("Sending h1->h2")
//[>log.ErrFatal(sendrcv_proc(h1, h2))<]
/*}*/

func init() {
	SimpleMessageType = RegisterPacketType(SimpleMessage{})
}

func NewTestTCPHost(port int) (*TCPHost, error) {
	addr := NewTCPAddress("127.0.0.1:" + strconv.Itoa(port))
	return NewTCPHost(addr)
}

// Returns a ServerIdentity out of the address
func NewTestServerIdentity(address Address) *ServerIdentity {
	kp := config.NewKeyPair(Suite)
	e := NewServerIdentity(kp.Public, address)
	return e
}

// SimpleMessage is just used to transfer one integer
type SimpleMessage struct {
	I int
}

var SimpleMessageType PacketTypeID

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

var statusMsgID = RegisterPacketType(statusMessage{})

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

func sendrcv_proc(from, to *Router) error {
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

func waitConnections(r *Router, sid *ServerIdentity) error {
	for i := 0; i < 10; i++ {
		c := r.connection(sid.ID)
		if c != nil {
			return nil
		}
		time.Sleep(WaitRetry)
	}
	return fmt.Errorf("Didn't see connection to %s in router", sid.Address)
}

func acceptAndClose(c Conn) {
	c.Close()
	return
}

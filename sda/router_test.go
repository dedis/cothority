package sda

import (
	"errors"
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
	case <-time.After(250 * time.Millisecond):
		t.Fatal("waited too long")
	}
}

func (t *TCPRouter) abortConnections() error {
	t.closeConnections()
	close(t.quitProcessMsg)
	return t.host.Close()
}

func TestTcpRouterReconnection(t *testing.T) {
	h1 := NewMockTcpRouter(2005)
	h2 := NewMockTcpRouter(2006)
	defer func() {
		h1.Close()
		h2.Close()
		// Let some time to tcp
		time.Sleep(250 * time.Millisecond)
	}()

	go h1.Run()
	go h2.Run()

	log.Lvl1("Sending h1->h2")
	log.ErrFatal(sendrcv_proc(h1, h2))
	log.Lvl1("Sending h2->h1")
	log.ErrFatal(sendrcv_proc(h2, h1))
	log.Lvl1("Closing h1")
	assert.Nil(t, h1.closeConnections())

	log.Lvl1("Listening again on h1")
	go h1.listen()
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

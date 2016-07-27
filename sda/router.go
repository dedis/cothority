package sda

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

type Router interface {
	Dispatcher

	// XXX TODO replace this by Route
	SendRaw(e *network.ServerIdentity, msg network.Body) error
	//	Route(id *network.ServerIdentity, msg network.Packet)
	Close() error

	// XXX TODO Feels like there's a lot of common goal for the next methods
	// that could maybe be factored together into something simpler...
	Tx() uint64
	Rx() uint64
	StatusReporter
	Address() string

	// TO REMOVE IDEALLY
	ListenAndBind()
	StartProcessMessages()
	// Real usage in sda.GenLocalHosts
	//	Connect(e *network.ServerIdentity) (network.SecureConn, error)
	// Real usage in sda.GenLocalHosts
	//	Connections() map[network.ServerIdentityID]network.SecureConn
	Listen()
}

type TcpRouter struct {
	// The TCPHost
	host           network.SecureHost
	serverIdentity *network.ServerIdentity
	suite          abstract.Suite
	connections    map[network.ServerIdentityID]network.SecureConn

	// chan of received messages - testmode
	networkChan chan network.Packet
	// We're about to close
	isClosing  bool
	closingMut sync.Mutex
	// lock associated to access network connections
	networkLock sync.RWMutex

	Dispatcher

	// working address is mostly for debugging purposes so we know what address
	// is known as right now
	workingAddress string
	// listening is a flag to tell whether this host is listening or not
	listening bool
	// whether processMessages has started
	processMessagesStarted bool
	// tell processMessages to quit
	ProcessMessagesQuit chan bool
}

func NewTcpRouter(e *network.ServerIdentity, pkey abstract.Scalar) *TcpRouter {
	return &TcpRouter{
		Dispatcher:          NewBlockingDispatcher(),
		workingAddress:      e.First(),
		connections:         make(map[network.ServerIdentityID]network.SecureConn),
		host:                network.NewSecureTCPHost(pkey, e),
		suite:               network.Suite,
		serverIdentity:      e,
		ProcessMessagesQuit: make(chan bool),
		networkChan:         make(chan network.Packet, 1),
	}
}

func (t *TcpRouter) Connections() map[network.ServerIdentityID]network.SecureConn {
	return t.connections

}

// SendRaw sends to an ServerIdentity without wrapping the msg into a SDAMessage
func (t *TcpRouter) SendRaw(e *network.ServerIdentity, msg network.Body) error {
	if msg == nil {
		return errors.New("Can't send nil-packet")
	}
	t.networkLock.RLock()
	c, ok := t.connections[e.ID]
	t.networkLock.RUnlock()
	if !ok {
		var err error
		c, err = t.Connect(e)
		if err != nil {
			return err
		}
	}

	log.Lvlf4("%s sends to %s msg: %+v", t.serverIdentity.Addresses, e, msg)
	var err error
	err = c.Send(context.TODO(), msg)
	if err != nil /*&& err != network.ErrClosed*/ {
		log.Lvl2("Couldn't send to", c.ServerIdentity().First(), ":", err, "trying again")
		c, err = t.Connect(e)
		if err != nil {
			return err
		}
		err = c.Send(context.TODO(), msg)
		if err != nil {
			return err
		}
	}
	log.Lvl5("Message sent")
	return nil
}

// listen starts listening for messages coming from any host that tries to
// contact this host. If 'wait' is true, it will try to connect to itself before
// returning.
func (t *TcpRouter) listen(wait bool) {
	log.Lvl3(t.serverIdentity.First(), "starts to listen")
	fn := func(c network.SecureConn) {
		log.Lvl3(t.workingAddress, "Accepted Connection from", c.Remote())
		// register the connection once we know it's ok
		t.registerConnection(c)
		t.handleConn(c)
	}
	go func() {
		log.Lvl4("Host listens on:", t.workingAddress)
		err := t.host.Listen(fn)
		if err != nil {
			log.Fatal("Couldn't listen on", t.workingAddress, ":", err)
		}
	}()
	if wait {
		for {
			log.Lvl4(t.serverIdentity.First(), "checking if listener is up")
			_, err := t.Connect(t.serverIdentity)
			if err == nil {
				log.Lvl4(t.serverIdentity.First(), "managed to connect to itself")
				break
			}
			time.Sleep(network.WaitRetry)
		}
	}
}

// ListenAndBind starts listening and returns once it could connect to itself.
// This can fail in the case of running inside a container or virtual machine
// using port-forwarding to an internal IP.
func (r *TcpRouter) ListenAndBind() {
	r.listen(true)
}

// Listen only starts listening and returns without waiting for the
// listening to be active.
func (r *TcpRouter) Listen() {
	r.listen(false)
}

// Connect takes an entity where to connect to
func (t *TcpRouter) Connect(id *network.ServerIdentity) (network.SecureConn, error) {
	var err error
	var c network.SecureConn
	// try to open connection
	c, err = t.host.Open(id)
	if err != nil {
		return nil, err
	}
	log.Lvl3("Host", t.workingAddress, "connected to", c.Remote())
	t.registerConnection(c)
	go t.handleConn(c)
	return c, nil
}

// Close shuts down all network connections and closes the listener.
func (t *TcpRouter) Close() error {

	t.closingMut.Lock()
	defer t.closingMut.Unlock()
	if t.isClosing {
		return errors.New("Already closing")
	}
	log.Lvl4(t.serverIdentity.First(), "Starts closing")
	t.isClosing = true
	if t.processMessagesStarted {
		// Tell ProcessMessages to quit
		close(t.ProcessMessagesQuit)
	}
	if err := t.closeConnections(); err != nil {
		return err
	}
	return nil
}

// CloseConnections only shuts down the network connections - used mainly
// for testing.
func (t *TcpRouter) closeConnections() error {
	t.networkLock.Lock()
	defer t.networkLock.Unlock()
	for _, c := range t.connections {
		log.Lvl4(t.serverIdentity.First(), "Closing connection", c, c.Remote(), c.Local())
		err := c.Close()
		if err != nil {
			log.Error(t.serverIdentity.First(), "Couldn't close connection", c)
			return err
		}
	}
	log.Lvl4(t.serverIdentity.First(), "Closing tcpHost")
	t.connections = make(map[network.ServerIdentityID]network.SecureConn)
	return t.host.Close()
}

// closeConnection closes a connection and removes it from the connections-map
// The t.networkLock must be taken.
func (t *TcpRouter) closeConnection(c network.SecureConn) error {
	t.networkLock.Lock()
	defer t.networkLock.Unlock()
	log.Lvl4(t.serverIdentity.First(), "Closing connection", c, c.Remote(), c.Local())
	err := c.Close()
	if err != nil {
		return err
	}
	delete(t.connections, c.ServerIdentity().ID)
	return nil
}

// StartProcessMessages start the processing of incoming messages.
// Mostly it used internally (by the cothority's simulation for instance).
// Protocol/simulation developers usually won't need it.
func (t *TcpRouter) StartProcessMessages() {
	// The networkLock.Unlock is in the processMessages-method to make
	// sure the goroutine started
	t.networkLock.Lock()
	t.processMessagesStarted = true
	t.ProcessMessagesQuit = make(chan bool)
	go t.processMessages()
}

// ProcessMessages checks if it is one of the messages for us or dispatch it
// to the corresponding instance.
// Our messages are:
// * SDAMessage - used to communicate between the Hosts
// * RequestTreeID - ask the parent for a given tree
// * SendTree - send the tree to the child
// * RequestPeerListID - ask the parent for a given peerList
// * SendPeerListID - send the tree to the child
func (t *TcpRouter) processMessages() {
	t.networkLock.Unlock()
	for {
		var data network.Packet
		select {
		case data = <-t.networkChan:
		case <-t.ProcessMessagesQuit:
			log.Lvl5(t.workingAddress, "Quitting ProcessMessages")
			return
		}
		log.Lvl4(t.workingAddress, "Message Received from", data.From, data.MsgType)
		switch data.MsgType {
		case network.ErrorType:
			log.Lvl3("Error from the network")
		default:
			// The dispatcher will call the appropriate processors for the
			// message
			if err := t.Dispatch(&data); err != nil {
				log.Lvl3("Unknown message received:", data, err)
			}
		}
	}
}

// Handle a connection => giving messages to the MsgChans
func (t *TcpRouter) handleConn(c network.SecureConn) {
	address := c.Remote()
	for {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		// This is for testing purposes only: if the connection is missing
		// in the map, we just return silently
		t.networkLock.Lock()
		_, cont := t.connections[c.ServerIdentity().ID]
		t.networkLock.Unlock()
		if !cont {
			log.Lvl3(t.workingAddress, "Quitting handleConn ", c.Remote(), " because entry is not there")
			return
		}
		// So the receiver can know about the error
		am.SetError(err)
		am.From = address
		log.Lvl5(t.workingAddress, "Got message", am)
		if err != nil {
			t.closingMut.Lock()
			log.Lvlf4("%+v got error (%+s) while receiving message (isClosing=%+v)",
				t.serverIdentity.First(), err, t.isClosing)
			t.closingMut.Unlock()
			if err == network.ErrClosed || err == network.ErrEOF || err == network.ErrTemp {
				log.Lvl4(t.serverIdentity.First(), c.Remote(), "quitting handleConn for-loop", err)
				t.closeConnection(c)
				return
			}
			log.Error(t.serverIdentity.Addresses, "Error with connection", address, "=>", err)
		} else {
			t.closingMut.Lock()
			if !t.isClosing {
				log.Lvl5(t.workingAddress, "Send message to networkChan", len(t.networkChan))
				t.networkChan <- am
			}
			t.closingMut.Unlock()
		}
	}
}

// registerConnection registers an ServerIdentity for a new connection, mapped with the
// real physical address of the connection and the connection itself
// it locks (and unlocks when done):  networkLock
func (t *TcpRouter) registerConnection(c network.SecureConn) {
	log.Lvl4(t.serverIdentity.First(), "registers", c.ServerIdentity().First())
	t.networkLock.Lock()
	defer t.networkLock.Unlock()
	id := c.ServerIdentity()
	_, okc := t.connections[id.ID]
	if okc {
		// TODO - we should catch this in some way
		log.Lvl3("Connection already registered", okc)
	}
	t.connections[id.ID] = c
}

// WaitForClose returns only once all connections have been closed
func (t *TcpRouter) WaitForClose() {
	if t.processMessagesStarted {
		<-t.ProcessMessagesQuit
	}
}

// Tx to implement monitor/CounterIO
func (t *TcpRouter) Tx() uint64 {
	return t.host.Tx()
}

// Rx to implement monitor/CounterIO
func (t *TcpRouter) Rx() uint64 {
	return t.host.Rx()
}

// Address is the address where this host is listening
func (t *TcpRouter) Address() string {
	return t.workingAddress
}

// GetStatus is a function that returns the status report of the server.
func (t *TcpRouter) GetStatus() Status {
	m := make(map[string]string)
	nbr := len(t.connections)
	remote := make([]string, nbr)
	iter := 0
	var rx uint64
	var tx uint64
	for _, c := range t.connections {
		remote[iter] = c.Remote()
		rx += c.Rx()
		tx += c.Tx()
		iter = iter + 1
	}
	m["Connections"] = strings.Join(remote, "\n")
	m["Host"] = t.Address()
	m["Total"] = strconv.Itoa(nbr)
	m["Packets_Received"] = strconv.FormatUint(rx, 10)
	m["Packets_Sent"] = strconv.FormatUint(tx, 10)
	return m
}

// XXX TODO EXPORTED FUNCTION TO BE REMOVED !
func (t *TcpRouter) AbortConnections() error {
	t.closeConnections()
	close(t.ProcessMessagesQuit)
	return t.host.Close()
}

func (t *TcpRouter) connection(e *network.ServerIdentity) network.SecureConn {
	t.networkLock.RLock()
	defer t.networkLock.RUnlock()
	c, _ := t.connections[e.ID]
	return c
}

// XXX To be removed (it causes more problem than we think, during the tests
// notably)
func (t *TcpRouter) Receive() network.Packet {
	data := <-t.networkChan
	log.Lvl5("Got message", data)
	return data
}

// MOCKING NETWORK ROUTER

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
	log.Lvl5(m.identity.First(), "Send msg", t.String(), "to", e.First())
	return nil
}

func (m *localRouter) Listen() {
	ready := make(chan bool)
	go func() {
		ready <- true
		for msg := range m.msgChan {
			log.Lvl5(m.Address(), "Received message", msg.MsgType, "from", msg.ServerIdentity.First())
			// XXX Do we need a go routine here ?
			if err := m.Dispatch(msg); err != nil {
				log.Lvl4(m.Address(), "Error dispatching:", err)
			}
		}
	}()
	<-ready
}

func (m *localRouter) ServerIdentity() *network.ServerIdentity {
	return m.identity
}
func (m *localRouter) Close() error {
	close(m.msgChan)
	return nil
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

func (l *localRouter) StartProcessMessages() {
	l.Listen()
}

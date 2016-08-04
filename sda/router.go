package sda

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

// Router is an abstraction to represent the bridge between the communication
// layer (network, channels etc) and the logical/processing layer (overlay &
// protocols, services etc). It is a duplex communication link (send/receive)
// from/to other router of the same type.
// Typically, for deployement you would use a tcpRouter so it opens tcp ports
// and communicate through tcp connections. For testing, there is a localRouter
// which pass all messages through channels going to other localRouter.
// For the Router to dispatch messages to your struct, you need to register a
// `Processor` (see the `Dispatcher` interface in processor.go).
type Router interface {
	// Run will start the Router:
	//  * Accepting new connections
	//  * Dispatching  incoming messages
	// It is a blocking call which will return until Close() is called.
	Run()
	// Close will stop the Router from running and will close all connections.
	// It makes the Run() returns.
	Close() error

	// Router is a Dispatcher so you can register any Processor to it. Every
	// messages coming to this Router will be dispatched accordingly to its
	// registered Processors.
	Dispatcher

	// Send will send the message msg to e.
	Send(e *network.ServerIdentity, msg network.Body) error

	// XXX Feels like there's a lot of common goal for the next methods
	// that could maybe be factored together into something simpler...
	Tx() uint64
	Rx() uint64
	StatusReporter
	Address() string
}

// TCPRouter is a Router implementation that uses TCP connections to communicate
// to different hosts. It manages automatically the connection to hosts, the
// maintenance of the connections etc. It is believed to be thread-safe.
type TCPRouter struct {
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
	quitProcessMsg chan bool
	// tell Run() to stop
	closing chan bool
}

// NewTCPRouter returns a fresh Router which uses TCP connections to
// communicate.
func NewTCPRouter(e *network.ServerIdentity, pkey abstract.Scalar) *TCPRouter {
	return &TCPRouter{
		Dispatcher:     NewBlockingDispatcher(),
		workingAddress: e.First(),
		connections:    make(map[network.ServerIdentityID]network.SecureConn),
		host:           network.NewSecureTCPHost(pkey, e),
		suite:          network.Suite,
		serverIdentity: e,
		quitProcessMsg: make(chan bool),
		closing:        make(chan bool, 1), // buffered channel of 1 so Close() without
		// Run() before does not fail
		networkChan: make(chan network.Packet, 1),
	}
}

// Send sends to an ServerIdentity without wrapping the msg into a SDAMessage
func (t *TCPRouter) Send(e *network.ServerIdentity, msg network.Body) error {
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

// Run will start opening a tcp port and accepting connections. It is a blocking
// call. This function returns when an error occurs on the open port or when
// t.Stop() is called.
func (t *TCPRouter) Run() {
	// start processing messages
	go func() {
		t.networkLock.Lock()
		t.quitProcessMsg = make(chan bool)
		if t.processMessagesStarted {
			// we are already listening
			t.networkLock.Unlock()
			return
		}
		t.networkLock.Unlock()

		t.processMessages()
	}()
	// listen
	go func() {
		err := t.listen()
		if err != nil {
			log.Fatal("Error listening on", t.workingAddress, ":", err)
		}
	}()
	<-t.closing
}

func (t *TCPRouter) listen() error {
	// open port
	log.Lvl3(t.serverIdentity.First(), "starts to listen")
	fn := func(c network.SecureConn) {
		log.Lvl3(t.workingAddress, "Accepted Connection from", c.Remote())
		// register the connection once we know it's ok
		t.registerConnection(c)
		t.handleConn(c)
	}
	err := t.host.Listen(fn)
	if err != nil {
		log.Fatal("Error listening on", t.workingAddress, ":", err)
		return err
	}
	return nil
}

// processMessages checks if it is one of the messages for us or dispatch it
// to the corresponding instance.
// Our messages are:
// * SDAMessage - used to communicate between the Hosts
// * RequestTreeID - ask the parent for a given tree
// * SendTree - send the tree to the child
// * RequestPeerListID - ask the parent for a given peerList
// * SendPeerListID - send the tree to the child
func (t *TCPRouter) processMessages() {
	t.networkLock.Lock()
	t.processMessagesStarted = true
	log.Lvl5(t.workingAddress, "Starting Process Messages")
	t.networkLock.Unlock()
	for {
		var data network.Packet
		select {
		case <-t.quitProcessMsg:
			log.Lvl5(t.workingAddress, "Quitting ProcessMessages")
			t.networkLock.Lock()
			t.processMessagesStarted = false
			t.networkLock.Unlock()
			return
		case data = <-t.networkChan:
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

// Connect takes an entity where to connect to
func (t *TCPRouter) Connect(id *network.ServerIdentity) (network.SecureConn, error) {
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
func (t *TCPRouter) Close() error {
	t.closingMut.Lock()
	defer t.closingMut.Unlock()
	if t.isClosing {
		return errors.New("Already closing")
	}
	log.Lvl4(t.serverIdentity.First(), "Starts closing")
	t.isClosing = true
	// stop the Run
	t.closing <- true
	t.networkLock.Lock()
	if t.processMessagesStarted {
		// Tell ProcessMessages to quit
		close(t.quitProcessMsg)
	}
	t.networkLock.Unlock()
	// The tcp host is supposed to take care of the connection for us
	if err := t.host.Close(); err != nil {
		return err
	}
	return nil
}

// closeConnection closes a connection and removes it from the connections-map
// The t.networkLock must be taken.
func (t *TCPRouter) closeConnection(c network.SecureConn) error {
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

// Handle a connection => giving messages to the MsgChans
func (t *TCPRouter) handleConn(c network.SecureConn) {
	address := c.Remote()
	for {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
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
func (t *TCPRouter) registerConnection(c network.SecureConn) {
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

// Tx to implement monitor/CounterIO
func (t *TCPRouter) Tx() uint64 {
	return t.host.Tx()
}

// Rx to implement monitor/CounterIO
func (t *TCPRouter) Rx() uint64 {
	return t.host.Rx()
}

// Address is the address where this host is listening
func (t *TCPRouter) Address() string {
	return t.workingAddress
}

// GetStatus is a function that returns the status report of the server.
func (t *TCPRouter) GetStatus() Status {
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

func (t *TCPRouter) connection(e *network.ServerIdentity) network.SecureConn {
	t.networkLock.RLock()
	defer t.networkLock.RUnlock()
	c, _ := t.connections[e.ID]
	return c
}

// MOCKING NETWORK ROUTER
// localRelay defines the basic functionalities such as sending and
// receiving a message, locally. It is implemented by localRouter and
// localClient so both a Router and a Client can be emulated locally without
// opening any real connections.
type localRelay interface {
	send(e *network.ServerIdentity, msg network.Body) error
	receive(msg *network.Packet)
	serverIdentity() *network.ServerIdentity
}

// localRouterStore keeps tracks of all the mock routers
type localRelayStore struct {
	localRelays map[network.ServerIdentityID]localRelay
	mut         sync.Mutex
}

// localRouters is the store that keeps tracks of all opened local routers in a
// thread safe manner
var localRelays = localRelayStore{
	localRelays: make(map[network.ServerIdentityID]localRelay),
}

func (lrs *localRelayStore) Put(r localRelay) {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	lrs.localRelays[r.serverIdentity().ID] = r
}

// Get returns the router associated with this ServerIdentity. It returns nil if
// there is no localRouter associated with this ServerIdentity
func (lrs *localRelayStore) Get(id *network.ServerIdentity) localRelay {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	r, ok := lrs.localRelays[id.ID]
	if !ok {
		return nil
	}
	return r
}

func (lrs *localRelayStore) Len() int {
	lrs.mut.Lock()
	defer lrs.mut.Unlock()
	return len(lrs.localRelays)
}

// LocalRouter is a struct that implements the Router interface locally using
// channels and go routines.
type LocalRouter struct {
	Dispatcher
	identity *network.ServerIdentity
	// msgQueue is the channel where other localRouter communicate messages to
	// this localRouter.
	msgChan chan *network.Packet
	conns   *connsStore
}

// NewLocalRouter will return a fresh router using native go channels to communicate
// to others localRouter. Its purpose is mainly for easy testing without any
// trouble of opening / closing / waiting for the network socket ...
func NewLocalRouter(identity *network.ServerIdentity) *LocalRouter {
	r := &LocalRouter{
		Dispatcher: NewBlockingDispatcher(),
		identity:   identity,
		msgChan:    make(chan *network.Packet, 100),
		conns:      newConnsStore(),
	}
	localRelays.Put(r)
	return r
}

func (l *LocalRouter) serverIdentity() *network.ServerIdentity {
	return l.identity
}

func (l *LocalRouter) send(e *network.ServerIdentity, msg network.Body) error {
	return l.Send(e, msg)
}

// Send implements the Router interface
func (l *LocalRouter) Send(e *network.ServerIdentity, msg network.Body) error {
	r := localRelays.Get(e)
	if r == nil {
		return errors.New("No mock routers at this entity")
	}

	l.conns.Put(e.String())

	var body network.Body
	var val = reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	body = val.Interface()

	var typ = network.TypeFromData(body)
	nm := network.Packet{
		MsgType:        typ,
		Msg:            body,
		ServerIdentity: l.identity,
	}
	r.receive(&nm)
	return nil
}

func (l *LocalRouter) receive(msg *network.Packet) {
	l.msgChan <- msg
}

// Run will make the LocalRouter start listening on its incoming channel. It's a
// blocking call.
func (l *LocalRouter) Run() {
	for msg := range l.msgChan {
		l.conns.Put(msg.ServerIdentity.String())
		log.Lvl5(l.Address(), "Received message", msg.MsgType, "from", msg.ServerIdentity.First())
		if err := l.Dispatch(msg); err != nil {
			log.Lvl4(l.Address(), "Error dispatching:", err)
		}
	}
}

// Close implements the Router interface. It will stop the dispatching of
// incoming messages.
func (l *LocalRouter) Close() error {
	close(l.msgChan)
	return nil
}

// Tx implements the Router interface (mainly for compatibility reason with
// monitor.CounterIO which is needed for TcpRouter simulations)
func (l *LocalRouter) Tx() uint64 {
	return 0
}

// Rx implements the Router interface (mainly for compatibility reason with
// monitor.CounterIO which is needed for TcpRouter simulations)
func (l *LocalRouter) Rx() uint64 {
	return 0
}

// GetStatus implements the Router interface
func (l *LocalRouter) GetStatus() Status {
	m := make(map[string]string)
	m["Connections"] = strings.Join(l.conns.Get(), "\n")
	m["Host"] = l.Address()
	m["Total"] = strconv.Itoa(l.conns.Len())
	m["Packets_Received"] = strconv.FormatUint(0, 10)
	m["Packets_Sent"] = strconv.FormatUint(0, 10)
	return m
}

// Address implements the Router interface
func (l *LocalRouter) Address() string {
	return l.identity.First()
}

type connsStore struct {
	// conns keep tracks of to whom this local router sent something so it can
	// have a reasonable loooking report status in GetStatus
	conns map[string]bool
	sync.Mutex
}

func (cs *connsStore) Put(name string) {
	cs.Lock()
	defer cs.Unlock()
	cs.conns[name] = true
}
func (cs *connsStore) Get() []string {
	cs.Lock()
	defer cs.Unlock()
	var names []string
	for k := range cs.conns {
		names = append(names, k)
	}
	return names
}
func (cs *connsStore) Len() int {
	cs.Lock()
	defer cs.Unlock()
	return len(cs.conns)
}

func newConnsStore() *connsStore {
	return &connsStore{
		conns: make(map[string]bool),
	}
}

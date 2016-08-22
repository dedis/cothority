package sda

import (
	"errors"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
)

// Router is an abstraction to represent the bridge between the communication
// layer (network, channels etc) and the logical/processing layer (overlay &
// protocols, services etc). It is a duplex communication link (send/receive)
// from/to other routers of the same type.
// Typically, for deployment you would use a tcpRouter so it opens tcp ports
// and communicate through tcp connections. For testing, there is a LocalRouter
// which passes all messages through channels going to another LocalRouter.
// For the Router to dispatch messages to your struct, you need to register a
// `Processor` (see the `Dispatcher` interface in processor.go).
type Router interface {
	// Run will start the Router:
	//  * Accepting new connections
	//  * Dispatching  incoming messages
	// It is a blocking call which won't return until Close() is called
	Run()
	// Close will stop the Router from running and will close all connections.
	// It makes the Run() method returns.
	Close() error

	// Router is a Dispatcher so you can register any Processor to it. Every
	// messages coming to this Router will be dispatched according to its
	// registered Processors.
	Dispatcher

	// Send will send the message msg to e.
	Send(e *network.ServerIdentity, msg network.Body) error

	Tx() uint64
	Rx() uint64
	StatusReporter
	Address() string
}

// TCPRouter is a Router implementation that uses TCP connections to communicate
// to different hosts. It manages automatically the connection to hosts, the
// maintenance of the connections etc. It is supposed to be thread-safe.
type TCPRouter struct {
	// The TCPHost
	host           network.SecureHost
	serverIdentity *network.ServerIdentity
	suite          abstract.Suite
	connections    map[network.ServerIdentityID]network.SecureConn

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
		// buffered channel of 1 so Close() without
		// Run() before does not fail
		closing:     make(chan bool, 1),
		networkChan: make(chan network.Packet, 1),
	}
}

// Run will start opening a tcp port and accepting connections. It is a blocking
// call. This function returns when an error occurs on the open port or when
// t.Stop() is called.
func (t *TCPRouter) Run() {
	// start processing messages
	go func() {
		t.quitProcessMut.Lock()
		t.quitProcessMsg = make(chan bool)
		if t.processMessagesStarted {
			// we are already listening
			t.quitProcessMut.Unlock()
			return
		}
		t.quitProcessMut.Unlock()

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

// processMessages is receiving all the messages coming from the network and
// dispatches them to the Dispatcher.
func (t *TCPRouter) processMessages() {
	t.quitProcessMut.Lock()
	t.processMessagesStarted = true
	log.Lvl5(t.workingAddress, "Starting Process Messages")
	t.quitProcessMut.Unlock()
	for {
		var data network.Packet
		select {
		case <-t.quitProcessMsg:
			log.Lvl5(t.workingAddress, "Quitting ProcessMessages")
			t.quitProcessMut.Lock()
			t.processMessagesStarted = false
			t.quitProcessMut.Unlock()
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

func (t *TCPRouter) connection(e *network.ServerIdentity) network.SecureConn {
	t.networkLock.RLock()
	defer t.networkLock.RUnlock()
	c, _ := t.connections[e.ID]
	return c
}

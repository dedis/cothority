package sda

import (
	"errors"
	"sync"
	"time"

	"strings"

	"sort"

	"strconv"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
)

// Host is the structure responsible for holding information about the current
// state
type Host struct {
	// Our entity (i.e. identity over the network)
	ServerIdentity *network.ServerIdentity
	// Our private-key
	private abstract.Scalar
	// The TCPHost
	host network.SecureHost
	Dispatcher
	// Overlay handles the mapping from tree and entityList to ServerIdentity.
	// It uses tokens to represent an unique ProtocolInstance in the system
	overlay *Overlay
	// The open connections
	connections map[network.ServerIdentityID]network.SecureConn
	// chan of received messages - testmode
	networkChan chan network.Packet
	// The suite used for this Host
	suite abstract.Suite
	// We're about to close
	isClosing  bool
	closingMut sync.Mutex
	// lock associated to access network connections
	networkLock sync.RWMutex
	// lock associated to access trees
	treesLock sync.Mutex
	// listening is a flag to tell whether this host is listening or not
	listening bool
	// whether processMessages has started
	processMessagesStarted bool
	// tell processMessages to quit
	ProcessMessagesQuit chan bool

	serviceManager       *serviceManager
	statusReporterStruct *statusReporterStruct
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(si *network.ServerIdentity, pkey abstract.Scalar) *Host {
	log.Lvl4("Creating host at", si.Addresses)
	h := &Host{
		ServerIdentity:       si,
		Dispatcher:           NewBlockingDispatcher(),
		connections:          make(map[network.ServerIdentityID]network.SecureConn),
		host:                 network.NewSecureTCPHost(pkey, si),
		private:              pkey,
		suite:                network.Suite,
		networkChan:          make(chan network.Packet, 1),
		isClosing:            false,
		ProcessMessagesQuit:  make(chan bool),
		statusReporterStruct: newStatusReporterStruct(),
	}

	h.overlay = NewOverlay(h)
	h.serviceManager = newServiceManager(h, h.overlay)
	h.statusReporterStruct.RegisterStatusReporter("Status", h)
	return h
}

// listen starts listening for messages coming from any host that tries to
// contact this host. If 'wait' is true, it will try to connect to itself before
// returning.
func (h *Host) listen(wait bool) {
	log.Lvl3(h.ServerIdentity.First(), "starts to listen")
	idExchDone := make(chan bool)
	fn := func(c network.SecureConn) {
		log.Lvl3(h.Address(), "Accepted Connection from", c.Remote())
		// register the connection once we know it's ok
		h.registerConnection(c)
		if wait && c.ServerIdentity().Equal(h.ServerIdentity) {
			idExchDone <- true
		}
		h.handleConn(c)
	}
	log.Lvl4("Host listens on:", h.Address())
	err := h.host.Listen(fn)
	if err != nil {
		log.Fatal("Couldn't listen on", h.Address(), ":", err)
	}
	if wait {
		for {
			log.Lvl4(h.ServerIdentity.First(), "checking if listener is up")
			_, err := h.Connect(h.ServerIdentity)
			if err == nil {
				log.Lvl4(h.ServerIdentity.First(), "managed to connect to itself")
				break
			}
			time.Sleep(network.WaitRetry)
		}
		<-idExchDone
	}
}

// ListenAndBind starts listening and returns once it could connect to itself.
// This can fail in the case of running inside a container or virtual machine
// using port-forwarding to an internal IP.
func (h *Host) ListenAndBind() {
	h.listen(true)
}

// Listen only starts listening and returns without waiting for the
// listening to be active.
func (h *Host) Listen() {
	h.listen(false)
}

// Connect takes an entity where to connect to
func (h *Host) Connect(id *network.ServerIdentity) (network.SecureConn, error) {
	var err error
	var c network.SecureConn
	// try to open connection
	c, err = h.host.Open(id)
	if err != nil {
		return nil, err
	}
	log.Lvl3("Host", h.Address(), "connected to", c.Remote())
	h.registerConnection(c)
	go h.handleConn(c)
	return c, nil
}

// Close shuts down all network connections and closes the listener.
func (h *Host) Close() error {
	h.closingMut.Lock()
	defer h.closingMut.Unlock()
	if h.isClosing {
		return errors.New("Already closing")
	}
	log.Lvl4(h.ServerIdentity.First(), "Starts closing")
	h.isClosing = true
	if h.processMessagesStarted {
		// Tell ProcessMessages to quit
		close(h.ProcessMessagesQuit)
		close(h.networkChan)
	}
	if err := h.closeConnections(); err != nil {
		return err
	}
	h.overlay.Close()
	return nil
}

// CloseConnections only shuts down the network connections - used mainly
// for testing.
func (h *Host) closeConnections() error {
	h.networkLock.Lock()
	defer h.networkLock.Unlock()
	for name, c := range h.connections {
		log.Lvl4(h.ServerIdentity.First(), "Closing connection", name)
		err := c.Close()
		if err != nil {
			log.Error(h.ServerIdentity.First(), "Couldn't close connection", c)
			return err
		}
	}
	log.Lvl4(h.ServerIdentity.First(), "Closing tcpHost")
	h.connections = make(map[network.ServerIdentityID]network.SecureConn)
	return h.host.Close()
}

// closeConnection closes a connection and removes it from the connections-map
// The h.networkLock must be taken.
func (h *Host) closeConnection(c network.SecureConn) error {
	h.networkLock.Lock()
	defer h.networkLock.Unlock()
	log.Lvl4(h.ServerIdentity.First(), "Closing connection", c, c.Remote(), c.Local())
	err := c.Close()
	if err != nil {
		return err
	}
	delete(h.connections, c.ServerIdentity().ID)
	return nil
}

// SendRaw sends to an ServerIdentity without wrapping the msg into a SDAMessage
func (h *Host) SendRaw(si *network.ServerIdentity, msg network.Body) error {
	if msg == nil {
		return errors.New("Can't send nil-packet")
	}
	h.networkLock.RLock()
	c, ok := h.connections[si.ID]
	h.networkLock.RUnlock()
	if !ok {
		var err error
		c, err = h.Connect(si)
		if err != nil {
			return err
		}
	}

	log.Lvlf4("%s sends to %s over %s", h.ServerIdentity.Addresses,
		si.Addresses, c.Local())
	var err error
	err = c.Send(context.TODO(), msg)
	if err != nil /*&& err != network.ErrClosed*/ {
		log.Lvl2("Couldn't send to", c.ServerIdentity().First(), ":", err, "trying again")
		c, err = h.Connect(si)
		if err != nil {
			return err
		}
		err = c.Send(context.TODO(), msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartProcessMessages start the processing of incoming messages.
// Mostly it used internally (by the cothority's simulation for instance).
// Protocol/simulation developers usually won't need it.
func (h *Host) StartProcessMessages() {
	// The networkLock.Unlock is in the processMessages-method to make
	// sure the goroutine started
	h.networkLock.Lock()
	h.processMessagesStarted = true
	go h.processMessages()
}

// ProcessMessages checks if it is one of the messages for us or dispatch it
// to the corresponding instance.
// Our messages are:
// * SDAMessage - used to communicate between the Hosts
// * RequestTreeID - ask the parent for a given tree
// * SendTree - send the tree to the child
// * RequestPeerListID - ask the parent for a given peerList
// * SendPeerListID - send the tree to the child
func (h *Host) processMessages() {
	h.networkLock.Unlock()
	for {
		var data network.Packet
		select {
		case data = <-h.networkChan:
		case <-h.ProcessMessagesQuit:
			return
		}
		log.Lvl4(h.Address(), "Message Received from", data.From, data.MsgType)
		switch data.MsgType {
		case network.ErrorType:
			log.Lvl3("Error from the network")
		default:
			// The dispatcher will call the appropriate processors for the
			// message
			if err := h.Dispatch(&data); err != nil {
				log.Lvl3("Unknown message received:", data)
			}
		}
	}
}

// Handle a connection => giving messages to the MsgChans
func (h *Host) handleConn(c network.SecureConn) {
	address := c.Remote()
	for {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		// This is for testing purposes only: if the connection is missing
		// in the map, we just return silently
		h.networkLock.Lock()
		_, cont := h.connections[c.ServerIdentity().ID]
		h.networkLock.Unlock()
		if !cont {
			log.Lvl3(h.Address(), "Quitting handleConn ", c.Remote(), " because entry is not there")
			return
		}
		// So the receiver can know about the error
		am.SetError(err)
		am.From = address
		log.Lvl5("Got message", am)
		if err != nil {
			h.closingMut.Lock()
			log.Lvlf4("%+v got error (%+s) while receiving message (isClosing=%+v)",
				h.ServerIdentity.First(), err, h.isClosing)
			h.closingMut.Unlock()
			if err == network.ErrClosed || err == network.ErrEOF || err == network.ErrTemp {
				log.Lvl4(h.ServerIdentity.First(), c.Remote(), "quitting handleConn for-loop", err)
				h.closeConnection(c)
				return
			}
			log.Error(h.ServerIdentity.Addresses, "Error with connection", address, "=>", err)
		} else {
			h.closingMut.Lock()
			if !h.isClosing {
				h.networkChan <- am
			}
			h.closingMut.Unlock()
		}
	}
}

// registerConnection registers an ServerIdentity for a new connection, mapped with the
// real physical address of the connection and the connection itself
// it locks (and unlocks when done): entityListsLock and networkLock
func (h *Host) registerConnection(c network.SecureConn) {
	log.Lvl4(h.ServerIdentity.First(), "registers", c.ServerIdentity().First())
	h.networkLock.Lock()
	defer h.networkLock.Unlock()
	id := c.ServerIdentity()
	_, okc := h.connections[id.ID]
	if okc {
		// TODO - we should catch this in some way
		log.Lvl3("Connection already registered", okc)
	}
	h.connections[id.ID] = c
}

// AddTree registers the given Tree struct in the underlying overlay.
// Useful for unit-testing only.
// XXX probably move into the tests.
func (h *Host) AddTree(t *Tree) {
	h.overlay.RegisterTree(t)
}

// AddRoster registers the given Roster in the underlying overlay.
// Useful for unit-testing only.
// XXX probably move into the tests.
func (h *Host) AddRoster(el *Roster) {
	h.overlay.RegisterRoster(el)
}

// Suite can (and should) be used to get the underlying abstract.Suite.
// Currently the suite is hardcoded into the network library.
// Don't use network.Suite but Host's Suite function instead if possible.
func (h *Host) Suite() abstract.Suite {
	return h.suite
}

// SetupHostsMock can be used to create a Host mock for testing.
func SetupHostsMock(s abstract.Suite, addresses ...string) []*Host {
	var hosts []*Host
	for _, add := range addresses {
		h := newHostMock(s, add)
		h.ListenAndBind()
		h.StartProcessMessages()
		hosts = append(hosts, h)
	}
	return hosts
}

func newHostMock(s abstract.Suite, address string) *Host {
	kp := config.NewKeyPair(s)
	en := network.NewServerIdentity(kp.Public, address)
	return NewHost(en, kp.Secret)
}

// WaitForClose returns only once all connections have been closed
func (h *Host) WaitForClose() {
	if h.processMessagesStarted {
		select {
		case <-h.ProcessMessagesQuit:
		}
	}
}

// Tx to implement monitor/CounterIO
func (h *Host) Tx() uint64 {
	return h.host.Tx()
}

// Rx to implement monitor/CounterIO
func (h *Host) Rx() uint64 {
	return h.host.Rx()
}

// Address is the address where this host is listening
func (h *Host) Address() string {
	return h.host.WorkingAddress()
}

// GetStatus is a function that returns the status report of the server.
func (h *Host) GetStatus() Status {
	m := make(map[string]string)
	nbr := len(h.connections)
	remote := make([]string, nbr)
	iter := 0
	var rx uint64
	var tx uint64
	for _, c := range h.connections {
		remote[iter] = c.Remote()
		rx += c.Rx()
		tx += c.Tx()
		iter = iter + 1
	}
	m["Connections"] = strings.Join(remote, "\n")
	m["Host"] = h.Address()
	m["Total"] = strconv.Itoa(nbr)
	m["Packets_Received"] = strconv.FormatUint(rx, 10)
	m["Packets_Sent"] = strconv.FormatUint(tx, 10)
	a := ServiceFactory.RegisteredServicesName()
	sort.Strings(a)
	m["Available_Services"] = strings.Join(a, ",")

	return m
}

// GetService returns the service with the given name.
func (h *Host) GetService(name string) Service {
	return h.serviceManager.Service(name)
}

package sda

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
)

// Host is the structure responsible for holding information about the current
// state
type Host struct {
	// Our entity (i.e. identity over the network)
	Entity *network.Entity
	// Our private-key
	private abstract.Secret
	// The TCPHost
	host network.SecureHost
	// Overlay handles the mapping from tree and entityList to Entity.
	// It uses tokens to represent an unique ProtocolInstance in the system
	overlay *Overlay
	// The open connections
	connections map[network.EntityID]network.SecureConn
	// chan of received messages - testmode
	networkChan chan network.Message
	// The database of entities this host knows
	entities map[network.EntityID]*network.Entity
	// lock associated to access entityLists
	entityListsLock sync.RWMutex
	// treeMarshal that needs to be converted to Tree but host does not have the
	// entityList associated yet.
	// map from EntityList.ID => trees that use this entity list
	pendingTreeMarshal map[EntityListID][]*TreeMarshal
	// pendingSDAData are a list of message we received that does not correspond
	// to any local Tree or/and EntityList. We first request theses so we can
	// instantiate properly protocolInstance that will use these SDAData msg.
	pendingSDAs []*Data
	// The suite used for this Host
	suite abstract.Suite
	// We're about to close
	isClosing  bool
	closingMut sync.Mutex
	// lock associated to access network connections
	networkLock sync.Mutex
	// lock associated to access trees
	treesLock sync.Mutex
	// lock associated with pending TreeMarshal
	pendingTreeLock sync.Mutex
	// lock associated with pending SDAdata
	pendingSDAsLock sync.Mutex
	// working address is mostly for debugging purposes so we know what address
	// is known as right now
	workingAddress string
	// listening is a flag to tell whether this host is listening or not
	listening bool
	// whether processMessages has started
	processMessagesStarted bool
	// tell processMessages to quit
	ProcessMessagesQuit chan bool

	serviceStore *serviceStore
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(e *network.Entity, pkey abstract.Secret) *Host {
	h := &Host{
		Entity:              e,
		workingAddress:      e.First(),
		connections:         make(map[network.EntityID]network.SecureConn),
		entities:            make(map[network.EntityID]*network.Entity),
		pendingTreeMarshal:  make(map[EntityListID][]*TreeMarshal),
		pendingSDAs:         make([]*Data, 0),
		host:                network.NewSecureTCPHost(pkey, e),
		private:             pkey,
		suite:               network.Suite,
		networkChan:         make(chan network.Message, 1),
		isClosing:           false,
		ProcessMessagesQuit: make(chan bool),
	}

	h.overlay = NewOverlay(h)
	h.serviceStore = newServiceStore(h, h.overlay)
	return h
}

// listen starts listening for messages coming from any host that tries to
// contact this host. If 'wait' is true, it will try to connect to itself before
// returning.
func (h *Host) listen(wait bool) {
	dbg.Lvl3(h.Entity.First(), "starts to listen")
	fn := func(c network.SecureConn) {
		dbg.Lvl3(h.workingAddress, "Accepted Connection from", c.Remote())
		// register the connection once we know it's ok
		h.registerConnection(c)
		h.handleConn(c)
	}
	go func() {
		dbg.Lvl4("Host listens on:", h.workingAddress)
		err := h.host.Listen(fn)
		if err != nil {
			dbg.Fatal("Couldn't listen on", h.workingAddress, ":", err)
		}
	}()
	if wait {
		for {
			dbg.Lvl4(h.Entity.First(), "checking if listener is up")
			_, err := h.Connect(h.Entity)
			if err == nil {
				dbg.Lvl4(h.Entity.First(), "managed to connect to itself")
				break
			}
			time.Sleep(network.WaitRetry)
		}
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
func (h *Host) Connect(id *network.Entity) (network.SecureConn, error) {
	var err error
	var c network.SecureConn
	// try to open connection
	c, err = h.host.Open(id)
	if err != nil {
		return nil, err
	}
	dbg.Lvl3("Host", h.workingAddress, "connected to", c.Remote())
	h.registerConnection(c)
	go h.handleConn(c)
	return c, nil
}

// Close shuts down the listener
func (h *Host) Close() error {
	h.networkLock.Lock()
	defer h.networkLock.Unlock()

	h.closingMut.Lock()
	if h.isClosing {
		h.closingMut.Unlock()
		return errors.New("Already closing")
	}
	dbg.Lvl4(h.Entity.First(), "Starts closing")
	h.isClosing = true
	h.closingMut.Unlock()
	if h.processMessagesStarted {
		// Tell ProcessMessages to quit
		close(h.ProcessMessagesQuit)
	}
	for _, c := range h.connections {
		dbg.Lvl4(h.Entity.First(), "Closing connection", c)
		err := c.Close()
		if err != nil {
			dbg.Error(h.Entity.First(), "Couldn't close connection", c)
			return err
		}
	}
	dbg.Lvl4(h.Entity.First(), "Closing tcpHost")
	err := h.host.Close()
	h.connections = make(map[network.EntityID]network.SecureConn)
	h.overlay.Close()
	return err
}

// SendRaw sends to an Entity without wrapping the msg into a SDAMessage
func (h *Host) SendRaw(e *network.Entity, msg network.ProtocolMessage) error {
	if msg == nil {
		return errors.New("Can't send nil-packet")
	}
	h.entityListsLock.RLock()
	if _, ok := h.entities[e.ID]; !ok {
		dbg.Lvl4(h.Entity.First(), "Connecting to", e.Addresses)
		h.entityListsLock.RUnlock()
		// Connect to that entity
		_, err := h.Connect(e)
		if err != nil {
			return err
		}
	} else {
		h.entityListsLock.RUnlock()
	}
	var c network.SecureConn
	var ok bool
	h.networkLock.Lock()
	if c, ok = h.connections[e.ID]; !ok {
		h.networkLock.Unlock()
		return errors.New("Got no connection tied to this Entity")
	}
	h.networkLock.Unlock()

	dbg.Lvlf4("%s sends to %s msg: %+v", e, h.Entity.Addresses, msg)
	if err := c.Send(context.TODO(), msg); err != nil /*&& err != network.ErrClosed*/ {
		dbg.Error("ERROR Sending to", c.Entity().First(), ":", err)
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
		var err error
		var data network.Message
		select {
		case data = <-h.networkChan:
		case <-h.ProcessMessagesQuit:
			return
		}
		dbg.Lvl4(h.workingAddress, "Message Received from", data.From, data.MsgType)
		switch data.MsgType {
		case SDADataMessageID:
			sdaMsg := data.Msg.(Data)
			sdaMsg.Entity = data.Entity
			err := h.overlay.TransmitMsg(&sdaMsg)
			if err != nil {
				dbg.Error("ProcessSDAMessage returned:", err)
			}
			// A host has sent us a request to get a tree definition
		case RequestTreeMessageID:
			tid := data.Msg.(RequestTree).TreeID
			tree := h.overlay.Tree(tid)
			if tree != nil {
				err = h.SendRaw(data.Entity, tree.MakeTreeMarshal())
			} else {
				// XXX Take care here for we must verify at the other side that
				// the tree is Nil. Should we think of a way of sending back an
				// "error" ?
				err = h.SendRaw(data.Entity, (&Tree{}).MakeTreeMarshal())
			}
			// A Host has replied to our request of a tree
		case SendTreeMessageID:
			tm := data.Msg.(TreeMarshal)
			if tm.TreeId == TreeID(uuid.Nil) {
				dbg.Error("Received an empty Tree")
				continue
			}
			il := h.overlay.EntityList(tm.EntityListID)
			// The entity list does not exists, we should request that, too
			if il == nil {
				msg := &RequestEntityList{tm.EntityListID}
				if err := h.SendRaw(data.Entity, msg); err != nil {
					dbg.Error("Requesting EntityList in SendTree failed", err)
				}

				// put the tree marshal into pending queue so when we receive the
				// entitylist we can create the real Tree.
				h.addPendingTreeMarshal(&tm)
				continue
			}

			tree, err := tm.MakeTree(il)
			if err != nil {
				dbg.Error("Couldn't create tree:", err)
				continue
			}
			dbg.Lvl4("Received new tree")
			h.overlay.RegisterTree(tree)
			h.checkPendingSDA(tree)
			// Some host requested an EntityList
		case RequestEntityListMessageID:
			id := data.Msg.(RequestEntityList).EntityListID
			el := h.overlay.EntityList(id)
			if el != nil {
				err = h.SendRaw(data.Entity, el)
			} else {
				dbg.Lvl2("Requested entityList that we don't have")
				err := h.SendRaw(data.Entity, &EntityList{})
				if err != nil {
					dbg.Error("Couldn't send empty entity list from host:",
						h.Entity.String(),
						err)
				}
			}
			// Host replied to our request of entitylist
		case SendEntityListMessageID:
			il := data.Msg.(EntityList)
			if il.Id == EntityListID(uuid.Nil) {
				dbg.Lvl2("Received an empty EntityList")
			} else {
				h.overlay.RegisterEntityList(&il)
				// Check if some trees can be constructed from this entitylist
				h.checkPendingTreeMarshal(&il)
			}
			dbg.Lvl4("Received new entityList")
		case RequestID:
			r := data.Msg.(ClientRequest)
			h.processRequest(data.Entity, &r)
		case ServiceMessageID:
			m := data.Msg.(ServiceMessage)
			h.processServiceMessage(data.Entity, &m)
		default:
			dbg.Error("Sending error:", err)
		}
	}
}

func (h *Host) processServiceMessage(e *network.Entity, m *ServiceMessage) {
	// check if the target service is indeed existing
	s, ok := h.serviceStore.serviceByID(m.Service)
	if !ok {
		dbg.Error("Received a message for an unknown service", m.Service)
		// XXX TODO should reply with some generic response =>
		// 404 Service Unknown
		return
	}
	dbg.Lvl3("host", h.Address(), " => Dispatch request to ServiceMessage")
	s.ProcessServiceMessage(e, m)

}

func (h *Host) processRequest(e *network.Entity, r *ClientRequest) {
	// check if the target service is indeed existing
	s, ok := h.serviceStore.serviceByID(r.Service)
	if !ok {
		dbg.Error("Received a request for an unknown service", r.Service)
		// XXX TODO should reply with some generic response =>
		// 404 Service Unknown
		return
	}
	dbg.Lvl3("host", h.Address(), " => Dispatch request to Request")
	go s.ProcessClientRequest(e, r)
}

// sendSDAData marshals the inner msg and then sends a Data msg
// to the appropriate entity
func (h *Host) sendSDAData(e *network.Entity, sdaMsg *Data) error {
	b, err := network.MarshalRegisteredType(sdaMsg.Msg)
	if err != nil {
		typ := network.TypeFromData(sdaMsg.Msg)
		rtype := reflect.TypeOf(sdaMsg.Msg)
		var str string
		if typ == network.ErrorType {
			str = " Non registered Type !"
		} else {
			str = typ.String()
		}
		str += " (reflect= " + rtype.String()
		return fmt.Errorf("Error marshaling  message: %s  ( msg = %+v)", err.Error(), sdaMsg.Msg)
	}
	sdaMsg.MsgSlice = b
	sdaMsg.MsgType = network.TypeFromData(sdaMsg.Msg)
	// put to nil so protobuf won't encode it and there won't be any error on the
	// other side (because it doesn't know how to decode it)
	sdaMsg.Msg = nil
	dbg.Lvl4("Sending to", e.Addresses)
	return h.SendRaw(e, sdaMsg)
}

// Handle a connection => giving messages to the MsgChans
func (h *Host) handleConn(c network.SecureConn) {
	address := c.Remote()
	for {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		// So the receiver can know about the error
		am.SetError(err)
		am.From = address
		dbg.Lvl5("Got message", am)
		if err != nil {
			h.closingMut.Lock()
			dbg.Lvl4(fmt.Sprintf("%+v got error (%+s) while receiving message (isClosing=%+v)",
				h.Entity.First(), err, h.isClosing))
			h.closingMut.Unlock()
			if err == network.ErrClosed || err == network.ErrEOF || err == network.ErrTemp {
				dbg.Lvl4(h.Entity.First(), "quitting handleConn for-loop", err)
				return
			}
			dbg.Error(h.Entity.Addresses, "Error with connection", address, "=>", err)
		} else {
			h.networkChan <- am
		}
	}
}

// requestTree will ask for the tree the sdadata is related to.
// it will put the message inside the pending list of sda message waiting to
// have their trees.
func (h *Host) requestTree(e *network.Entity, sdaMsg *Data) error {
	h.addPendingSda(sdaMsg)
	treeRequest := &RequestTree{sdaMsg.To.TreeID}
	return h.SendRaw(e, treeRequest)
}

// addPendingSda simply append a sda message to a queue. This queue willbe
// checked each time we receive a new tree / entityList
func (h *Host) addPendingSda(sda *Data) {
	h.pendingSDAsLock.Lock()
	h.pendingSDAs = append(h.pendingSDAs, sda)
	h.pendingSDAsLock.Unlock()
}

// checkPendingSda is called each time we receive a new tree if there are some SDA
// messages using this tree. If there are, we can make an instance of a protocolinstance
// and give it the message!.
// NOTE: put that as a go routine so the rest of the processing messages are not
// slowed down, if there are many pending sda message at once (i.e. start many new
// protocols at same time)
func (h *Host) checkPendingSDA(t *Tree) {
	go func() {
		h.pendingSDAsLock.Lock()
		newPending := make([]*Data, 0)
		for _, msg := range h.pendingSDAs {
			// if this message references t
			if t.Id.Equals(msg.To.TreeID) {
				// instantiate it and go
				err := h.overlay.TransmitMsg(msg)
				if err != nil {
					dbg.Error("TransmitMsg failed:", err)
					continue
				}
			} else {
				newPending = append(newPending, msg)
			}
		}
		h.pendingSDAs = newPending
		h.pendingSDAsLock.Unlock()
	}()
}

// registerConnection registers a Entity for a new connection, mapped with the
// real physical address of the connection and the connection itself
// it locks (and unlocks when done): entityListsLock and networkLock
func (h *Host) registerConnection(c network.SecureConn) {
	dbg.Lvl4(h.Entity.First(), "registers", c.Entity().First())
	h.networkLock.Lock()
	h.entityListsLock.Lock()
	defer h.networkLock.Unlock()
	defer h.entityListsLock.Unlock()
	id := c.Entity()
	h.entities[c.Entity().ID] = id
	h.connections[c.Entity().ID] = c
}

// addPendingTreeMarshal adds a treeMarshal to the list.
// This list is checked each time we receive a new EntityList
// so trees using this EntityList can be constructed.
func (h *Host) addPendingTreeMarshal(tm *TreeMarshal) {
	h.pendingTreeLock.Lock()
	var sl []*TreeMarshal
	var ok bool
	// initiate the slice before adding
	if sl, ok = h.pendingTreeMarshal[tm.EntityListID]; !ok {
		sl = make([]*TreeMarshal, 0)
	}
	sl = append(sl, tm)
	h.pendingTreeMarshal[tm.EntityListID] = sl
	h.pendingTreeLock.Unlock()
}

// checkPendingTreeMarshal is called each time we add a new EntityList to the
// system. It checks if some treeMarshal use this entityList so they can be
// converted to Tree.
func (h *Host) checkPendingTreeMarshal(el *EntityList) {
	h.pendingTreeLock.Lock()
	sl, ok := h.pendingTreeMarshal[el.Id]
	if !ok {
		// no tree for this entitty list
		return
	}
	for _, tm := range sl {
		tree, err := tm.MakeTree(el)
		if err != nil {
			dbg.Error("Tree from EntityList failed")
			continue
		}
		// add the tree into our "database"
		h.overlay.RegisterTree(tree)
	}
	h.pendingTreeLock.Unlock()
}

// AddTree registers the given Tree struct in the underlying overlay.
// Useful for unit-testing only.
// XXX probably move into the tests.
func (h *Host) AddTree(t *Tree) {
	h.overlay.RegisterTree(t)
}

// AddEntityList registers the given EntityList in the underlying overlay.
// Useful for unit-testing only.
// XXX probably move into the tests.
func (h *Host) AddEntityList(el *EntityList) {
	h.overlay.RegisterEntityList(el)
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
	en := network.NewEntity(kp.Public, address)
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

// Tx() to implement monitor/CounterIO
func (h *Host) Tx() uint64 {
	return h.host.Tx()
}

// Rx() to implement monitor/CounterIO
func (h *Host) Rx() uint64 {
	return h.host.Rx()
}

// Address is the addres where this host is listening
func (h *Host) Address() string {
	return h.workingAddress
}

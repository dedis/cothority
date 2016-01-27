/*
Implementation of the Secure Distributed API - main module

Node takes care about
* the network
* pre-parsing incoming packets
* instantiating ProtocolInstances
* passing packets to ProtocolInstances

*/

package sda

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
)

/*
Host is the structure responsible for holding information about the current
 state
*/
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
	connections map[uuid.UUID]network.SecureConn
	// chan of received messages - testmode
	networkChan chan network.NetworkMessage
	// The database of entities this host knows
	entities map[uuid.UUID]*network.Entity
	// treeMarshal that needs to be converted to Tree but host does not have the
	// entityList associated yet.
	// map from EntityList.ID => trees that use this entity list
	pendingTreeMarshal map[uuid.UUID][]*TreeMarshal
	// pendingSDAData are a list of message we received that does not correspond
	// to any local tree or/and entitylist. We first request theses so we can
	// instantiate properly protocolinstance that will use these SDAData msg.
	pendingSDAs []*SDAData
	// The suite used for this Host
	suite abstract.Suite
	// closed channel to notifiy the connections that we close
	closed chan bool
	// lock associated to access network connections
	// and to access entities also.
	networkLock *sync.Mutex
	// lock associated to access entityLists
	entityListsLock *sync.Mutex
	// lock associated to access trees
	treesLock *sync.Mutex
	// lock associated with pending TreeMarshal
	pendingTreeLock *sync.Mutex
	// lock associated with pending SDAdata
	pendingSDAsLock *sync.Mutex
	// working address is mostly for debugging purposes so we know what address
	// is known as right now
	workingAddress string
	// listening is a flag to tell wether this host is listening or not
	listening bool
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(e *network.Entity, pkey abstract.Secret) *Host {
	h := &Host{
		Entity:             e,
		workingAddress:     e.First(),
		connections:        make(map[uuid.UUID]network.SecureConn),
		entities:           make(map[uuid.UUID]*network.Entity),
		pendingTreeMarshal: make(map[uuid.UUID][]*TreeMarshal),
		pendingSDAs:        make([]*SDAData, 0),
		host:               network.NewSecureTcpHost(pkey, e),
		private:            pkey,
		suite:              network.Suite,
		networkChan:        make(chan network.NetworkMessage, 1),
		closed:             make(chan bool),
		networkLock:        &sync.Mutex{},
		entityListsLock:    &sync.Mutex{},
		treesLock:          &sync.Mutex{},
		pendingTreeLock:    &sync.Mutex{},
		pendingSDAsLock:    &sync.Mutex{},
	}

	h.overlay = NewOverlay(h)
	return h
}

// NewHostKey creates a new host only from the ip-address and port-number. This
// is useful in testing and deployment for measurements
func NewHostKey(address string) (*Host, abstract.Secret) {
	keypair := config.NewKeyPair(network.Suite)
	entity := network.NewEntity(keypair.Public, address)
	return NewHost(entity, keypair.Secret), keypair.Secret
}

// Listen starts listening for messages coming from any host that tries to
// contact this entity / host
func (h *Host) Listen() {
	fn := func(c network.SecureConn) {
		dbg.Lvl3(h.workingAddress, "Accepted Connection from", c.Remote())
		// register the connection once we know it's ok
		h.registerConnection(c)
		h.handleConn(c)
	}
	go func() {
		dbg.Lvl3("Listening in", h.workingAddress)
		err := h.host.Listen(fn)
		if err != nil {
			dbg.Fatal("Couldn't listen in", h.workingAddress, ":", err)
		}
	}()
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
	h.registerConnection(c)
	dbg.Lvl2("Host", h.workingAddress, "connected to", c.Remote())
	go h.handleConn(c)
	return c, nil
}

// Close shuts down the listener
func (h *Host) Close() error {
	time.Sleep(time.Millisecond * 100)
	h.networkLock.Lock()
	var err error
	err = h.host.Close()
	h.connections = make(map[uuid.UUID]network.SecureConn)
	close(h.closed)
	h.networkLock.Unlock()
	return err
}

// SendRaw sends to an Entity without wrapping the msg into a SDAMessage
func (h *Host) SendRaw(e *network.Entity, msg network.ProtocolMessage) error {
	if msg == nil {
		return errors.New("Can't send nil-packet")
	}
	if _, ok := h.entities[e.Id]; !ok {
		// Connect to that entity
		_, err := h.Connect(e)
		if err != nil {
			return err
		}
	}
	var c network.SecureConn
	var ok bool
	if c, ok = h.connections[e.Id]; !ok {
		return errors.New("Got no connection tied to this Entity")
	}
	dbg.Lvl4("Sending to", e)
	c.Send(context.TODO(), msg)
	return nil
}

// ProcessMessages checks if it is one of the messages for us or dispatch it
// to the corresponding instance.
// Our messages are:
// * SDAMessage - used to communicate between the Hosts
// * RequestTreeID - ask the parent for a given tree
// * SendTree - send the tree to the child
// * RequestPeerListID - ask the parent for a given peerList
// * SendPeerListID - send the tree to the child
func (h *Host) ProcessMessages() {
	for {
		var err error
		data := h.receive()
		dbg.Lvl3("Message Received from", data.From)
		switch data.MsgType {
		case SDADataMessage:
			err := h.processSDAMessage(&data)
			if err != nil {
				dbg.Error("ProcessSDAMessage returned:", err)
			}
		// A host has sent us a request to get a tree definition
		case RequestTreeMessage:
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
		case SendTreeMessage:
			tm := data.Msg.(TreeMarshal)
			if tm.NodeId == uuid.Nil {
				dbg.Error("Received an empty Tree")
				continue
			}
			il := h.overlay.EntityList(tm.EntityId)
			// The entity list does not exists, we should request for that too
			if il == nil {
				msg := &RequestEntityList{tm.EntityId}
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
		case RequestEntityListMessage:
			id := data.Msg.(RequestEntityList).EntityListID
			el := h.overlay.EntityList(id)
			if el != nil {
				err = h.SendRaw(data.Entity, el)
			} else {
				dbg.Lvl2("Requested entityList that we don't have")
				h.SendRaw(data.Entity, &EntityList{})
			}
		// Host replied to our request of entitylist
		case SendEntityListMessage:
			il := data.Msg.(EntityList)
			if il.Id == uuid.Nil {
				dbg.Lvl2("Received an empty EntityList")
			} else {
				h.overlay.RegisterEntityList(&il)
				// Check if some trees can be constructed from this entitylist
				h.checkPendingTreeMarshal(&il)
			}
			dbg.Lvl4("Received new entityList")
		default:
			dbg.Error("Didn't recognize message", data.MsgType)
		}
		if err != nil {
			dbg.Error("Sending error:", err)
		}
	}
}

// sendSDAData marshals the inner msg and then sends a SDAData msg
// to the appropriate entity
func (h *Host) sendSDAData(e *network.Entity, sdaMsg *SDAData) error {
	b, err := network.MarshalRegisteredType(sdaMsg.Msg)
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}
	sdaMsg.MsgSlice = b
	sdaMsg.MsgType = network.TypeFromData(sdaMsg.Msg)
	// put to nil so protobuf won't encode it and there won't be any error on the
	// other side (because it doesn't know how to decode it)
	sdaMsg.Msg = nil
	return h.SendRaw(e, sdaMsg)
}

// Receive will return the value of the communication-channel, unmarshalling
// the SDAMessage. Receive is called in ProcessMessages as it takes directly
// the message from the networkChan, and pre-processes the SDAMessage
func (h *Host) receive() network.NetworkMessage {
	data := <-h.networkChan
	if data.MsgType == SDADataMessage {
		sda := data.Msg.(SDAData)
		t, msg, err := network.UnmarshalRegisteredType(sda.MsgSlice, data.Constructors)
		if err != nil {
			dbg.Error("Error while marshalling inner message of SDAData:", err)
		}
		// Put the msg into SDAData
		sda.MsgType = t
		sda.Msg = msg
		// Write back the Msg in appplicationMessage
		data.Msg = sda
		dbg.Lvlf3("SDA-Message is: %+v", sda.Msg)
	}
	return data
}

// Handle a connection => giving messages to the MsgChans
func (h *Host) handleConn(c network.SecureConn) {
	address := c.Remote()
	msgChan := make(chan network.NetworkMessage)
	errorChan := make(chan error)
	doneChan := make(chan bool)
	go func() {
		for {
			select {
			case <-doneChan:
				dbg.Lvl3("Closing", c)
				return
			default:
				ctx := context.TODO()
				am, err := c.Receive(ctx)
				// So the receiver can know about the error
				am.SetError(err)
				am.From = address
				if err != nil {
					errorChan <- err
				} else {
					msgChan <- am
				}
			}
		}
	}()
	for {
		select {
		case <-h.closed:
			doneChan <- true
		case am := <-msgChan:
			dbg.Lvl3("Putting message into networkChan from", am.From)
			h.networkChan <- am
		case e := <-errorChan:
			if e == network.ErrClosed || e == network.ErrEOF {
				return
			}
			dbg.Error("Error with connection", address, "=> error", e)
		case <-time.After(timeOut):
			dbg.Error("Timeout with connection", address)
		}
	}
}

// Dispatch SDA message looks if we have all the info to rightly dispatch the
// packet such as the protocol id and the topology id and the protocol instance
// id
func (h *Host) processSDAMessage(am *network.NetworkMessage) error {
	sdaMsg := am.Msg.(SDAData)
	t, msg, err := network.UnmarshalRegisteredType(sdaMsg.MsgSlice, network.DefaultConstructors(network.Suite))
	if err != nil {
		dbg.Error("Error unmarshaling embedded msg in SDAMessage", err)
	}
	// Set the right type and msg
	sdaMsg.MsgType = t
	sdaMsg.Msg = msg
	sdaMsg.Entity = am.Entity

	return h.overlay.TransmitMsg(&sdaMsg)
}

// requestTree will ask for the tree the sdadata is related to.
// it will put the message inside the pending list of sda message waiting to
// have their trees.
func (h *Host) requestTree(e *network.Entity, sdaMsg *SDAData) error {
	h.addPendingSda(sdaMsg)
	treeRequest := &RequestTree{sdaMsg.To.TreeID}
	return h.SendRaw(e, treeRequest)
}

// addPendingSda simply append a sda message to a queue. This queue willbe
// checked each time we receive a new tree / entityList
func (h *Host) addPendingSda(sda *SDAData) {
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
		for i := range h.pendingSDAs {
			// if this message references t
			if uuid.Equal(t.Id, h.pendingSDAs[i].To.TreeID) {
				// instantiate it and go
				err := h.overlay.TransmitMsg(h.pendingSDAs[i])
				if err != nil {
					dbg.Error("TransmitMsg failed:", err)
					continue
				}
			}
		}
		h.pendingSDAsLock.Unlock()
	}()
}

// registerConnection registers a Entity for a new connection, mapped with the
// real physical address of the connection and the connection itself
func (h *Host) registerConnection(c network.SecureConn) {
	h.networkLock.Lock()
	id := c.Entity()
	h.entities[c.Entity().Id] = id
	h.connections[c.Entity().Id] = c
	h.networkLock.Unlock()
}

// addPendingTreeMarshal adds a treeMarshal to the list.
// This list is checked each time we receive a new EntityList
// so trees using this EntityList can be constructed.
func (h *Host) addPendingTreeMarshal(tm *TreeMarshal) {
	h.pendingTreeLock.Lock()
	var sl []*TreeMarshal
	var ok bool
	// initiate the slice before adding
	if sl, ok = h.pendingTreeMarshal[tm.EntityId]; !ok {
		sl = make([]*TreeMarshal, 0)
	}
	sl = append(sl, tm)
	h.pendingTreeMarshal[tm.EntityId] = sl
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

func (h *Host) AddTree(t *Tree) {
	h.overlay.RegisterTree(t)
}

func (h *Host) AddEntityList(el *EntityList) {
	h.overlay.RegisterEntityList(el)
}

func (h *Host) Suite() abstract.Suite {
	return h.suite
}

func (h *Host) Private() abstract.Secret {
	return h.private
}

func (h *Host) StartNewNode(protoID uuid.UUID, tree *Tree) (*Node, error) {
	return h.overlay.StartNewNode(protoID, tree)
}

func SetupHostsMock(s abstract.Suite, addresses ...string) []*Host {
	var hosts []*Host
	for _, add := range addresses {
		h := newHostMock(s, add)
		h.Listen()
		go h.ProcessMessages()
		hosts = append(hosts, h)
	}
	return hosts
}

func newHostMock(s abstract.Suite, address string) *Host {
	kp := cliutils.KeyPair(s)
	en := network.NewEntity(kp.Public, address)
	return NewHost(en, kp.Secret)
}

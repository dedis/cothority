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
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"sync"
	"time"
)

/*
Host is the structure responsible for holding information about the current
 state
*/
type Host struct {
	// Our entity (i.e. identity over the network)
	Entity *network.Entity
	// the working address is set when the Host will start listen
	// When a listening adress found in the entity works, workingAddress is
	// set to that address
	workingAddress string
	// The TCPHost
	host network.SecureHost
	// The open connections
	connections map[uuid.UUID]network.SecureConn
	// and the lock associated to access them
	networkLock *sync.Mutex
	// The database of entities this host knows
	entities map[uuid.UUID]*network.Entity
	// The entityLists used for building the trees
	entityLists map[uuid.UUID]*EntityList
	// all trees known to this Host
	trees map[uuid.UUID]*Tree
	// Our private-key
	private abstract.Secret
	// The suite used for this Host
	suite abstract.Suite
	// slice of received messages - testmode
	networkChan chan network.ApplicationMessage
	// instances linked to their ID and their ProtocolID
	instances map[uuid.UUID]ProtocolInstance
	// closed channel to notifiy the connections that we close
	closed chan bool
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(id *network.Entity, pkey abstract.Secret, host network.SecureHost) *Host {
	n := &Host{
		Entity:         id,
		workingAddress: id.First(),
		networkLock:    &sync.Mutex{},
		connections:    make(map[uuid.UUID]network.SecureConn),
		entities:       make(map[uuid.UUID]*network.Entity),
		trees:          make(map[uuid.UUID]*Tree),
		entityLists:    make(map[uuid.UUID]*EntityList),
		host:           host,
		private:        pkey,
		suite:          network.Suite,
		networkChan:    make(chan network.ApplicationMessage, 1),
		instances:      make(map[uuid.UUID]ProtocolInstance),
		closed:         make(chan bool),
	}
	return n
}

// Listen starts listening for messages coming from any host that tries to
// contact this entity / host
func (n *Host) Listen() {
	fn := func(c network.SecureConn) {
		dbg.Lvl3(n.workingAddress, "Accepted Connection from", c.Remote())
		// register the connection once we know it's ok
		n.registerConnection(c)
		n.handleConn(c)
	}
	go func() {
		dbg.Lvl3("Listening in", n.workingAddress)
		err := n.host.Listen(fn)
		if err != nil {
			dbg.Fatal("Couldn't listen in", n.workingAddress, ":", err)
		}
	}()
}

// Connect takes an entity where to connect to
func (n *Host) Connect(id *network.Entity) (network.SecureConn, error) {
	var err error
	var c network.SecureConn
	// try to open connection
	c, err = n.host.Open(id)
	if err != nil {
		return nil, err
	}
	n.registerConnection(c)
	dbg.Lvl2("Host", n.workingAddress, "connected to", c.Remote())
	go n.handleConn(c)
	return c, nil
}

// Close shuts down the listener
func (n *Host) Close() error {
	n.networkLock.Lock()
	for _, c := range n.connections {
		dbg.Lvl3("Closing connection", c)
		c.Close()
	}
	err := n.host.Close()
	n.connections = make(map[uuid.UUID]network.SecureConn)
	close(n.closed)
	n.networkLock.Unlock()
	return err
}

// SendToRaw sends to an Entity without wrapping the msg into a SDAMessage
func (n *Host) SendToRaw(id *network.Entity, msg network.ProtocolMessage) error {
	if msg == nil {
		return fmt.Errorf("Can't send nil-packet")
	}
	if _, ok := n.entities[id.Id]; !ok {
		return fmt.Errorf("SendToEntity received a non-saved entity")
	}
	var c network.SecureConn
	var ok bool
	if c, ok = n.connections[id.Id]; !ok {
		return fmt.Errorf("Got no connection tied to this Entity")
	}
	c.Send(context.TODO(), msg)
	return nil
}

// SendMsgTo wraps the message to send in an SDAMessage and sends it to the
// appropriate entity
func (n *Host) SendMsgTo(id *network.Entity, msg network.ProtocolMessage) error {
	sdaMsg := &SDAData{
		ProtoID:    uuid.Nil,
		InstanceID: uuid.Nil,
	}
	b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}
	sdaMsg.MsgSlice = b
	sdaMsg.MsgType = network.TypeFromData(msg)
	return n.SendToRaw(id, sdaMsg)
}

// Receive will return the value of the communication-channel, unmarshalling
// the SDAMessage. Receive is called in ProcessMessages as it takes directly
// the message from the networkChan, and pre-process the SDAMessage
func (n *Host) Receive() network.ApplicationMessage {
	data := <-n.networkChan
	if data.MsgType == SDADataMessage {
		sda := data.Msg.(SDAData)
		t, msg, _ := network.UnmarshalRegisteredType(sda.MsgSlice, data.Constructors)
		// Put the msg into SDAData
		sda.MsgType = t
		sda.Msg = msg
		// Write back the Msg in appplicationMessage
		data.Msg = sda
		dbg.Lvl3("SDA-Message is:", sda)
	}
	return data
}

// ProcessMessages checks if it is one of the messages for us or dispatch it
// to the corresponding instance.
// Our messages are:
// * SDAMessage - used to communicate between the Hosts
// * RequestTreeID - ask the parent for a given tree
// * SendTree - send the tree to the child
// * RequestPeerListID - ask the parent for a given peerList
// * SendPeerListID - send the tree to the child
func (n *Host) ProcessMessages() {
	for {
		var err error
		data := n.Receive()
		dbg.Lvl3("Message Received:", data)
		switch data.MsgType {
		case SDADataMessage:
			n.processSDAMessage(&data)
		// A host has sent us a request to get a tree definition
		case RequestTreeMessage:
			tid := data.Msg.(RequestTree).TreeID
			tree, ok := n.trees[tid]
			if ok {
				err = n.SendToRaw(data.Entity, tree.MakeTreeMarshal())
			} else {
				// XXX Take care here for we must verify at the other side that
				// the tree is Nil. Should we think of a way of sending back an
				// "error" ?
				err = n.SendToRaw(data.Entity, (&Tree{}).MakeTreeMarshal())
			}
		// A Host has replied to our request of a tree
		case SendTreeMessage:
			tm := data.Msg.(TreeMarshal)
			if tm.Node == uuid.Nil {
				dbg.Error("Received an empty Tree")
				continue
			}
			il, ok := n.GetEntityList(tm.Entity)
			if !ok {
				dbg.Error("EntityList-id doesn't exist")
				continue
			}
			tree, err := tm.MakeTree(il)
			if err != nil {
				dbg.Error("Couldn't create tree:", err)
				continue
			}
			n.AddTree(tree)
		// Some host requested an EntityList
		case RequestEntityListMessage:
			id := data.Msg.(RequestEntityList).EntityListID
			il, ok := n.entityLists[id]
			if ok {
				err = n.SendToRaw(data.Entity, il)
			} else {
				err = n.SendToRaw(data.Entity, &EntityList{})
			}
		// Host replied to our request of entitylist
		case SendEntityListMessage:
			il := data.Msg.(EntityList)
			if il.Id == uuid.Nil {
				dbg.Error("Received an empty EntityList")
			}
			n.AddEntityList(&il)
		default:
			dbg.Error("Didn't recognize message", data.MsgType)
		}
		if err != nil {
			dbg.Error("Sending error:", err)
		}
	}
}

// AddEntityList stores the peer-list for further usage
func (n *Host) AddEntityList(il *EntityList) {
	if _, ok := n.entityLists[il.Id]; ok {
		dbg.Lvl2("Added EntityList with same ID")
	}
	n.entityLists[il.Id] = il
}

// AddTree stores the tree for further usage
func (n *Host) AddTree(t *Tree) {
	if _, ok := n.trees[t.Id]; ok {
		dbg.Lvl2("Added Tree with same ID")
	}
	n.trees[t.Id] = t
}

// GetEntityList returns the EntityList
func (n *Host) GetEntityList(id uuid.UUID) (*EntityList, bool) {
	il, ok := n.entityLists[id]
	return il, ok
}

// GetTree returns the TreeList
func (n *Host) GetTree(id uuid.UUID) (*Tree, bool) {
	t, ok := n.trees[id]
	return t, ok
}

var timeOut = 30 * time.Second

// Handle a connection => giving messages to the MsgChans
func (n *Host) handleConn(c network.SecureConn) {
	address := c.Remote()
	msgChan := make(chan network.ApplicationMessage)
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
		case <-n.closed:
			doneChan <- true
		case am := <-msgChan:
			dbg.Lvl3("Putting message into networkChan:", am)
			n.networkChan <- am
		case e := <-errorChan:
			if e == network.ErrClosed {
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
func (n *Host) processSDAMessage(am *network.ApplicationMessage) error {
	sda := am.Msg.(SDAData)
	t, msg, err := network.UnmarshalRegisteredType(sda.MsgSlice, network.DefaultConstructors(n.Suite()))
	if err != nil {
		dbg.Error("Error unmarshaling embedded msg in SDAMessage", err)
	}
	// Set the right type and msg
	sda.MsgType = t
	sda.Msg = msg
	dbg.Lvl3("Processing SDA-message", sda)
	/*
		TODO
		if !ProtocolExists(sda.ProtoID) {
			return fmt.Errorf("Protocol does not exists")
		}
	*/
	ip, ok := n.instances[sda.InstanceID]
	if !ok {
		// XXX What to do here ? create a new instance or just drop ?
		return fmt.Errorf("Instance Protocol not existing YET")
	}
	// Dispatch the message to the right instance !
	ip.Dispatch(&sda)
	return nil
}

// registerConnection registers a Entity for a new connection, mapped with the
// real physical address of the connection and the connection itself
func (n *Host) registerConnection(c network.SecureConn) {
	n.networkLock.Lock()
	id := c.Entity()
	n.entities[c.Entity().Id] = id
	n.connections[c.Entity().Id] = c
	n.networkLock.Unlock()
}

// Suite returns the suite used by the host
// NOTE for the moment the suite is fixed for the host and any protocols
// instance.
func (n *Host) Suite() abstract.Suite {
	return n.suite
}

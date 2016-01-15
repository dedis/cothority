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
	"github.com/dedis/cothority/lib/cliutils"
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
	// Our private-key
	private abstract.Secret
	// The TCPHost
	host network.SecureHost
	// mapper is used to uniquely identify instances + helpers so protocol
	// instances can send easily msg
	mapper *protocolMapper
	// The open connections
	connections map[uuid.UUID]network.SecureConn
	// chan of received messages - testmode
	networkChan chan network.ApplicationMessage
	// The database of entities this host knows
	entities map[uuid.UUID]*network.Entity
	// The entityLists used for building the trees
	entityLists map[uuid.UUID]*EntityList
	// all trees known to this Host
	trees map[uuid.UUID]*Tree
	// TreeNode that this host represents mapped by their respective TreeID
	treeNodes map[uuid.UUID]*TreeNode
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
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(id *network.Entity, pkey abstract.Secret, host network.SecureHost) *Host {
	n := &Host{
		Entity:             id,
		workingAddress:     id.First(),
		connections:        make(map[uuid.UUID]network.SecureConn),
		entities:           make(map[uuid.UUID]*network.Entity),
		trees:              make(map[uuid.UUID]*Tree),
		treeNodes:          make(map[uuid.UUID]*TreeNode),
		entityLists:        make(map[uuid.UUID]*EntityList),
		pendingTreeMarshal: make(map[uuid.UUID][]*TreeMarshal),
		pendingSDAs:        make([]*SDAData, 0),
		host:               host,
		private:            pkey,
		suite:              network.Suite,
		networkChan:        make(chan network.ApplicationMessage, 1),
		closed:             make(chan bool),
		networkLock:        &sync.Mutex{},
		entityListsLock:    &sync.Mutex{},
		treesLock:          &sync.Mutex{},
		pendingTreeLock:    &sync.Mutex{},
		pendingSDAsLock:    &sync.Mutex{},
	}

	n.mapper = newProtocolMapper(n)
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

// Send is the main function protocol instance must use in order to send a
// message accross the network. A PI must first give its assigned Token, then
// the Entity where it want to send the message then the msg. The message will
// be trasnformed into a SDAData message automatically.
func (n *Host) Send(tok *Token, id *network.Entity, msg network.ProtocolMessage) error {
	if n.mapper.Instance(tok) == nil {
		return fmt.Errorf("No protocol instance registered with this token.")
	}
	sda := &SDAData{
		Token: *tok,
		Msg:   msg,
	}
	return n.sendSDAData(id, sda)
}

// StartNewProtocol starts a new protocol by instantiating a instance of that
// protocol and then call Start on it.
func (n *Host) StartNewProtocol(protocolID uuid.UUID, treeID uuid.UUID) (ProtocolInstance, error) {
	// check everything exists
	if !ProtocolExists(protocolID) {
		return nil, fmt.Errorf("Protocol does not exists")
	}
	var tree *Tree
	var ok bool
	n.treesLock.Lock()
	if tree, ok = n.trees[treeID]; !ok {
		return nil, fmt.Errorf("TreeId does not exists")
	}
	n.treesLock.Unlock()

	// instantiate
	token := &Token{
		ProtocolID:   protocolID,
		EntityListID: tree.IdList.Id,
		TreeID:       treeID,
		// Host is handling the generation of protocolInstanceID
		InstanceID: cliutils.NewRandomUUID(),
	}
	// instantiate protocol instance
	pi, err := n.protocolInstantiate(token)
	if err != nil {
		return nil, err
	}

	// start it
	pi.Start()
	return pi, nil
}

// ProtocolInstantiate creates a new instance of a protocol given by it's name
func (n *Host) protocolInstantiate(tok *Token) (ProtocolInstance, error) {
	p, ok := protocols[tok.ProtocolID]
	if !ok {
		return nil, errors.New("Protocol doesn't exist")
	}
	tree, ok := n.GetTree(tok.TreeID)
	if !ok {
		return nil, errors.New("Tree does not exists")
	}
	if _, ok := n.GetEntityList(tok.EntityListID); !ok {
		return nil, errors.New("EntityList does not exists")
	}
	pi := p(n, tree, tok)
	n.mapper.RegisterProtocolInstance(pi, tok)
	return pi, nil
}

// sendSDAData do its marshalling of the inner msg and then sends a SDAData msg
// to the  appropriate entity
func (n *Host) sendSDAData(id *network.Entity, sdaMsg *SDAData) error {
	b, err := network.MarshalRegisteredType(sdaMsg.Msg)
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}
	sdaMsg.MsgSlice = b
	sdaMsg.MsgType = network.TypeFromData(sdaMsg.Msg)
	// put to nil so protobuf wont encode it and there wont be any error on the
	// other side (because it don't know how to encode it)
	sdaMsg.Msg = nil
	return n.SendToRaw(id, sdaMsg)
}

// Receive will return the value of the communication-channel, unmarshalling
// the SDAMessage. Receive is called in ProcessMessages as it takes directly
// the message from the networkChan, and pre-process the SDAMessage
func (n *Host) receive() network.ApplicationMessage {
	data := <-n.networkChan
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
		data := n.receive()
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
			if tm.NodeId == uuid.Nil {
				dbg.Error("Received an empty Tree")
				continue
			}
			il, ok := n.GetEntityList(tm.EntityId)
			// The entity list does not exists, we should request for that too
			if !ok {
				msg := &RequestEntityList{tm.EntityId}
				if err := n.SendToRaw(data.Entity, msg); err != nil {
					dbg.Error("Requesting EntityList in SendTree failed", err)
				}

				// put the tree marshal into pending queue so when we receive the
				// entitylist we can create the real Tree.
				n.addPendingTreeMarshal(&tm)
				continue
			}

			tree, err := tm.MakeTree(il)
			if err != nil {
				dbg.Error("Couldn't create tree:", err)
				continue
			}
			n.AddTree(tree)
			n.checkPendingSDA(tree)
		// Some host requested an EntityList
		case RequestEntityListMessage:
			id := data.Msg.(RequestEntityList).EntityListID
			il, ok := n.entityLists[id]
			if ok {
				err = n.SendToRaw(data.Entity, il)
			} else {
				dbg.Error("Requested entityList that we don't have")
				n.SendToRaw(data.Entity, &EntityList{})
			}
		// Host replied to our request of entitylist
		case SendEntityListMessage:
			il := data.Msg.(EntityList)
			if il.Id == uuid.Nil {
				dbg.Error("Received an empty EntityList")
			}
			n.AddEntityList(&il)
			// Check if some trees can be constructed from this entitylist
			n.checkPendingTreeMarshal(&il)
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
	n.entityListsLock.Lock()
	if _, ok := n.entityLists[il.Id]; ok {
		dbg.Lvl2("Added EntityList with same ID")
	}
	n.entityLists[il.Id] = il
	n.entityListsLock.Unlock()
}

// AddTree stores the tree for further usage
// IT also calls checkPendingSDA so we can now instantiate protocol instance
// using this tree
func (n *Host) AddTree(t *Tree) {
	n.treesLock.Lock()
	if _, ok := n.trees[t.Id]; ok {
		dbg.Lvl2("Added Tree with same ID")
	}
	n.trees[t.Id] = t
	// find ourself in this treeID
	var treeNode *TreeNode
	fn := func(d int, tn *TreeNode) {
		if tn.Entity.Equal(n.Entity) {
			treeNode = tn
		}
	}
	t.Root.Visit(0, fn)
	if treeNode == nil {
		dbg.Error("We did not find ourself in the added tree :s WEIRD")
	} else {
		n.treeNodes[t.Id] = treeNode
	}
	n.treesLock.Unlock()
	n.checkPendingSDA(t)
}

// GetEntityList returns the EntityList
func (n *Host) GetEntityList(id uuid.UUID) (*EntityList, bool) {
	n.entityListsLock.Lock()
	il, ok := n.entityLists[id]
	n.entityListsLock.Unlock()
	return il, ok
}

// GetTree returns the TreeList
func (n *Host) GetTree(id uuid.UUID) (*Tree, bool) {
	n.treesLock.Lock()
	t, ok := n.trees[id]
	n.treesLock.Unlock()
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
func (n *Host) processSDAMessage(am *network.ApplicationMessage) error {
	sda := am.Msg.(SDAData)
	t, msg, err := network.UnmarshalRegisteredType(sda.MsgSlice, network.DefaultConstructors(n.Suite()))
	if err != nil {
		dbg.Error("Error unmarshaling embedded msg in SDAMessage", err)
	}
	// Set the right type and msg
	sda.MsgType = t
	sda.Msg = msg
	sda.Entity = am.Entity
	if !ProtocolExists(sda.Token.ProtocolID) {
		return fmt.Errorf("Protocol does not exists from token")
	}
	// do we have the entitylist ? if not, ask for it.
	if _, ok := n.GetEntityList(sda.Token.EntityListID); !ok {
		dbg.Lvl2("Will ask for entityList + tree from token")
		return n.requestTree(am.Entity, &sda)
	}
	if _, ok := n.GetTree(sda.Token.TreeID); !ok {
		dbg.Lvl2("Will ask for tree from token")
		return n.requestTree(am.Entity, &sda)
	}
	// If pi does not exists, then instantiate it !
	if !n.mapper.Exists(sda.Token.Id()) {
		_, err := n.protocolInstantiate(&sda.Token)
		if err != nil {
			return err
		}
	}

	if n.mapper.DispatchToInstance(&sda) {
		return fmt.Errorf("Instance Protocol not existing YET")
	}
	return nil
}

// requestTree will ask for the tree the sdadata is related to.
// it will put the message inside the pending list of sda message waiting to
// have their trees.
func (n *Host) requestTree(e *network.Entity, sda *SDAData) error {
	n.addPendingSda(sda)
	treeRequest := &RequestTree{sda.Token.TreeID}
	return n.SendToRaw(e, treeRequest)
}

// addPendingSda simply append a sda message to a queue. This queue willbe
// checked each time we receive a new tree / entityList
func (n *Host) addPendingSda(sda *SDAData) {
	n.pendingSDAsLock.Lock()
	n.pendingSDAs = append(n.pendingSDAs, sda)
	n.pendingSDAsLock.Unlock()
}

// checkPendingSda check each time we receive a new tree that there are some SDA
// message using this tree . If there are, we can isntantite a protocolinstance
// and give it the message!
// NOTE: put that as a go routine so the rest of the processing messages are not
// slowed down, if there are many pending sda message at once (i.e. start many new
// protocol at same time)
func (n *Host) checkPendingSDA(t *Tree) {
	go func() {
		n.pendingSDAsLock.Lock()
		for i := range n.pendingSDAs {
			dbg.Lvl2("Check pending SDA")
			// if this message referes to this tree
			if uuid.Equal(t.Id, n.pendingSDAs[i].Token.TreeID) {
				// instantiate it and go !
				_, err := n.protocolInstantiate(&n.pendingSDAs[i].Token)
				if err != nil {
					dbg.Error("Instantite a protocol failed (should not happen)", err)
					continue
				}
				if !n.mapper.DispatchToInstance(n.pendingSDAs[i]) {
					dbg.Lvl2("dispatching did not work")
				}
			}
		}
		n.pendingSDAsLock.Unlock()
	}()
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

// addPendingTreeMarshal adds a treeMarshal to the list.
// This list is checked each time we receive a new EntityList
// so trees using this EntityList can be constructed.
func (n *Host) addPendingTreeMarshal(tm *TreeMarshal) {
	n.pendingTreeLock.Lock()
	var sl []*TreeMarshal
	var ok bool
	// initiate the slice before adding
	if sl, ok = n.pendingTreeMarshal[tm.EntityId]; !ok {
		sl = make([]*TreeMarshal, 0)
	}
	sl = append(sl, tm)
	n.pendingTreeMarshal[tm.EntityId] = sl
	n.pendingTreeLock.Unlock()
}

// checkPendingTreeMarshal is called each time we add a new EntityList to the
// system. It checks if some treeMarshal use this entityList so they can be
// converted to Tree.
func (n *Host) checkPendingTreeMarshal(el *EntityList) {
	n.pendingTreeLock.Lock()
	var sl []*TreeMarshal
	var ok bool
	if sl, ok = n.pendingTreeMarshal[el.Id]; !ok {
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
		n.AddTree(tree)
	}
	n.pendingTreeLock.Unlock()
}

// TreeNode returns the TreeNode that this hosts represents in the Tree that has
// id treeID. Nil if none.
func (n *Host) TreeNode(treeID uuid.UUID) *TreeNode {
	n.treesLock.Lock()
	tn, ok := n.treeNodes[treeID]
	n.treesLock.Unlock()
	if !ok {
		return nil
	}
	return tn
}

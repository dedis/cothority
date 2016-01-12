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
	// Our identity
	Identity *network.Identity
	// the working address is set when the Host will start listen
	// When a listening adress found in the identity works, workingAddress is
	// set to that address
	workingAddress string
	// The TCPHost
	host network.SecureHost
	// The open connections
	connections map[string]network.SecureConn
	// and the locks
	networkLock *sync.Mutex
	// The database of identities this host knows
	identities map[uuid.UUID]*network.Identity
	// The identityLists used for building the trees
	identityLists map[uuid.UUID]*IdentityList
	// all trees known to this Host
	trees map[uuid.UUID]*Tree
	// mapping from identity to physical addresses in connection
	identityToAddress map[uuid.UUID]string
	// identity mutex
	identityLock *sync.Mutex
	// Our private-key
	private abstract.Secret
	// The suite used for this Host
	suite abstract.Suite
	// slice of received messages - testmode
	networkChan chan MessageInfo
	// instances linked to their ID and their ProtocolID
	instances map[uuid.UUID]ProtocolInstance
	// closed channel to notifiy the connections that we close
	closed chan bool
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(id *network.Identity, pkey abstract.Secret, host network.SecureHost) *Host {
	n := &Host{
		Identity:          id,
		identityToAddress: make(map[uuid.UUID]string),
		workingAddress:    id.First(),
		networkLock:       &sync.Mutex{},
		identityLock:      &sync.Mutex{},
		connections:       make(map[string]network.SecureConn),
		identities:        make(map[uuid.UUID]*network.Identity),
		trees:             make(map[uuid.UUID]*Tree),
		identityLists:     make(map[uuid.UUID]*IdentityList),
		host:              host,
		private:           pkey,
		suite:             network.Suite,
		networkChan:       make(chan MessageInfo, 1),
		instances:         make(map[uuid.UUID]ProtocolInstance),
		closed:            make(chan bool),
	}
	return n
}

// Listen starts listening for messages coming from parent(up)
// each time a connection request is made, we receive first its identity then
// we handle the message using HandleConn.
// It will try each address in the Host identity until one works.
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

// Connect takes an identity where to connect to
func (n *Host) Connect(id *network.Identity) (network.SecureConn, error) {
	var err error
	var c network.SecureConn
	// try to open connection
	c, err = n.host.Open(*id)
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
	n.connections = make(map[string]network.SecureConn)
	close(n.closed)
	n.networkLock.Unlock()
	return err
}

// SendToRaw sends to an Identity by trying the different addresses tied
// to this id
// TODO: make this private - but used in test
func (n *Host) SendToRaw(id *network.Identity, msg network.ProtocolMessage) error {
	if msg == nil {
		return fmt.Errorf("Can't send nil-packet")
	}
	if _, ok := n.identities[id.Id]; !ok {
		return fmt.Errorf("SendToIdentity received a non-saved identity")
	}
	var addr string
	var ok bool
	if addr, ok = n.identityToAddress[id.Id]; !ok {
		return fmt.Errorf("No connection for this identity")
	}
	// additional verification - might be skipped
	n.networkLock.Lock()
	if c, ok := n.connections[addr]; !ok {
		n.networkLock.Unlock()
		return fmt.Errorf("No connection to this address!", n.connections)
	} else {
		// we got the connection
		n.networkLock.Unlock()
		dbg.Lvl3("Sending data", msg, "to", c.Remote())
		return c.Send(context.TODO(), msg)
	}
	return nil
}

// SendMsgTo wraps the message to send in an SDAMessage and sends it to the
// appropriate identity
func (n *Host) SendMsgTo(id *network.Identity, msg network.ProtocolMessage) error {
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
// the SDAMessage
func (n *Host) Receive() MessageInfo {
	msgInfo := <-n.networkChan
	var data = msgInfo.Data
	if msgInfo.Data.MsgType == SDADataMessage {
		sda := data.Msg.(SDAData)
		t, msg, _ := network.UnmarshalRegisteredType(sda.MsgSlice, data.Constructors)
		sda.MsgType = t
		sda.Msg = msg
		// As these are not pointers, we need to write it back
		data.Msg = sda
		msgInfo.Data = data
		dbg.Lvl3("SDA-Message is:", sda)
	}
	return msgInfo
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
		msgInfo := n.Receive()
		dbg.Lvl3("Message Received:", msgInfo.Data)
		switch msgInfo.Data.MsgType {
		case SDADataMessage:
			n.processSDAMessage(msgInfo.Id, &msgInfo.Data)
		case RequestTreeMessage:
			tid := msgInfo.Data.Msg.(RequestTree).TreeID
			tree, ok := n.trees[tid]
			if ok {
				err = n.SendToRaw(msgInfo.Id, tree.MakeTreeMarshal())
			} else {
				err = n.SendToRaw(msgInfo.Id, (&Tree{}).MakeTreeMarshal())
			}
		case SendTreeMessage:
			tm := msgInfo.Data.Msg.(TreeMarshal)
			if tm.Node == uuid.Nil {
				dbg.Error("Received an empty Tree")
				continue
			}
			il, ok := n.GetIdentityList(tm.Identity)
			if !ok {
				dbg.Error("IdentityList-id doesn't exist")
				continue
			}
			tree, err := tm.MakeTree(il)
			if err != nil {
				dbg.Error("Couldn't create tree:", err)
				continue
			}
			n.AddTree(tree)
		case RequestIdentityListMessage:
			id := msgInfo.Data.Msg.(RequestIdentityList).IdentityListID
			il, ok := n.identityLists[id]
			if ok {
				err = n.SendToRaw(msgInfo.Id, il)
			} else {
				err = n.SendToRaw(msgInfo.Id, &IdentityList{})
			}
		case SendIdentityListMessage:
			il := msgInfo.Data.Msg.(IdentityList)
			if il.Id == uuid.Nil {
				dbg.Error("Received an empty IdentityList")
			}
			n.AddIdentityList(&il)
		default:
			dbg.Error("Didn't recognize message", msgInfo.Data.MsgType)
		}
		if err != nil {
			dbg.Error("Sending error:", err)
		}
	}
}

// AddIdentityList stores the peer-list for further usage
func (n *Host) AddIdentityList(il *IdentityList) {
	n.identityLists[il.Id] = il
}

// AddTree stores the tree for further usage
func (n *Host) AddTree(t *Tree) {
	n.trees[t.Id] = t
}

// GetIdentityList returns the IdentityList
func (n *Host) GetIdentityList(id uuid.UUID) (*IdentityList, bool) {
	il, ok := n.identityLists[id]
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
	id := c.Identity()
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
			n.networkChan <- MessageInfo{Id: &id, Data: am}
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
func (n *Host) processSDAMessage(id *network.Identity, am *network.ApplicationMessage) error {
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

// registerConnection registers a Identity for a new connection, mapped with the
// real physical address of the connection and the connection itself
func (n *Host) registerConnection(c network.SecureConn) {
	n.networkLock.Lock()
	id := c.Identity()
	n.identities[c.Identity().Id] = &id
	n.identityToAddress[id.Id] = c.Remote()
	n.connections[c.Remote()] = c
	n.networkLock.Unlock()
}

// Suite returns the suite used by the host
// NOTE for the moment the suite is fixed for the host and any protocols
// instance.
func (n *Host) Suite() abstract.Suite {
	return n.suite
}

// MessageInfo is used to communicate the identity tied to a message when we
// receive messages
type MessageInfo struct {
	Id   *network.Identity
	Data network.ApplicationMessage
}

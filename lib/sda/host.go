/*
Implementation of the Secure Distributed API - main module

Node takes care about
* the network
* pre-parsing incoming packets
* instantiating ProtocolInstances
* passing packets to ProtocolInstances

Basically you can open connections to some addresses, and listen on some address.
Each time a packet is received is passed through ProcessMessages()
When it is a SDAMessage, that means it is destined to a ProtocolInstance, and that dispatching is done in processSDAMessages
To register a new protocol instance just call AddProtocolInstance()
For the dispatching to work, the packet sent must address the right protocolID, the right protocolInstanceID
and the right topologyID it relies on.
*/

package sda

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
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
	Identity *Identity
	// the working address is set when the Host will start listen
	// When a listening adress found in the identity works, workingAddress is
	// set to that address
	workingAddress string
	// The TCPHost
	host network.Host
	// The open connections
	connections map[string]network.Conn
	// and the locks
	networkLock *sync.Mutex
	// The database of identities this host knows
	identities map[UUID]*Identity
	// mapping from identity to physical addresses in connection
	identityToAddress map[UUID]string
	// identity mutex
	identityLock *sync.Mutex
	// Our private-key
	private abstract.Secret
	// The suite used for this Host
	suite abstract.Suite
	// slice of received messages - testmode
	networkChan chan MessageInfo
	// instances linked to their ID and their ProtocolID
	instances map[UUID]ProtocolInstance
	// all trees known to this Host
	trees map[UUID]Tree
	// closed channel to notifiy the connections that we close
	closed chan bool
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(id *Identity, pkey abstract.Secret, host network.Host) *Host {
	n := &Host{
		Identity:          id,
		identityToAddress: make(map[UUID]string),
		workingAddress:    id.First(),
		networkLock:       &sync.Mutex{},
		identityLock:      &sync.Mutex{},
		connections:       make(map[string]network.Conn),
		identities:        make(map[UUID]*Identity),
		host:              host,
		private:           pkey,
		suite:             network.Suite,
		networkChan:       make(chan MessageInfo, 1),
		instances:         make(map[UUID]ProtocolInstance),
		closed:            make(chan bool),
	}
	return n
}

// Start listening for messages coming from parent(up)
// each time a connection request is made, we receive first its identity then
// we handle the message using HandleConn
// NOTE Listen will try each address in the Host identity until one works ;)
func (n *Host) Listen() {
	fn := func(c network.Conn) {
		ctx := context.TODO()
		// receive the identity of the remote peer
		am, err := c.Receive(ctx)
		if err != nil || am.MsgType != IdentityType {
			dbg.Lvl2(n.workingAddress, "Error receiving identity from connection", c.Remote())
		}
		id := am.Msg.(Identity)
		var addr string
		if addr = id.First(); addr == "" {
			dbg.Error("Received a connection with Identity with NO addresses")
			return
		}
		dbg.Lvl3(n.workingAddress, "Accepted Connection from", addr)
		// register the connection once we know it's ok
		n.registerConnection(&id, addr, c)
		n.handleConn(&id, addr, c)
	}
	errChan := make(chan error)
	var stop bool
	// Try every addresses
	for _, addr := range n.Identity.Addresses {
		if stop {
			break
		}
		go func() {
			err := n.host.Listen(addr, fn)
			if err != nil {
				errChan <- err
			}
		}()
		select {
		// error while listening on this address
		case e := <-errChan:
			dbg.Lvl2("Unable to listen with address", addr, "=>err", e)
			// no error, we listen it's ok !
		case <-time.After(2 * time.Second):
			dbg.Lvl2(addr, "is listening !")
			stop = true
			n.workingAddress = addr
		}
	}
}

// Connect takes an identity where the next Host is
// It will try every addresses in the Identity
func (n *Host) Connect(id *Identity) (network.Conn, error) {
	var err error
	var c network.Conn
	for _, addr := range id.Addresses {
		// try to open connection
		c, err = n.host.Open(addr)
		if err != nil {
			continue
		}
		// Send our Identity so the remote host knows who we are
		if err := c.Send(context.TODO(), n.Identity); err != nil {
			return nil, err
		}
		n.registerConnection(id, addr, c)
		dbg.Lvl2("Host", n.workingAddress, "connected to", addr)
		go n.handleConn(id, addr, c)
	}
	return c, nil
}

// Close shuts down the listener
func (n *Host) Close() error {
	n.networkLock.Lock()
	err := n.host.Close()
	n.connections = make(map[string]network.Conn)
	close(n.closed)
	n.networkLock.Unlock()
	return err
}

// SendToIdentity sends to an Identity by trying the differents addresses tied
// to this id
// XXX SHould use that function in public and put SendTo in private
func (n *Host) SendTo(id *Identity, msg network.ProtocolMessage) error {
	if _, ok := n.identities[id.ID()]; !ok {
		return fmt.Errorf("SendToIdentity received an non-saved identity")
	}
	var addr string
	var ok bool
	if addr, ok = n.identityToAddress[id.ID()]; !ok {
		return fmt.Errorf("No connection for this identity")
	}
	// additionnal verification - might be skipped
	n.networkLock.Lock()
	if c, ok := n.connections[addr]; !ok {
		n.networkLock.Unlock()
		return fmt.Errorf("No connection to this address!", n.connections)
	} else {
		// we got the connection
		n.networkLock.Unlock()
		sdaMsg := &SDAMessage{
			ProtoID:    "",
			InstanceID: "",
		}
		b, err := network.MarshalRegisteredType(msg)
		if err != nil {
			return fmt.Errorf("Error marshaling  message: %s", err.Error())
		}
		sdaMsg.DataSlice = b
		sdaMsg.MsgType = network.TypeFromData(msg)
		dbg.Lvl3("Sending data", sdaMsg, "to", c.Remote())
		return c.Send(context.TODO(), sdaMsg)
	}
	return nil
}

// Receive will return the value of the communication-channel or an error
// if there has been nothing received during 2 seconds.
// XXX Should we do that? Consider a node that takes time to compute or to
// aggregate information from its children, 2 secs is low. Maybe set a higher
// timeout. And also, this is a general timeout, for every connections, do we
// suppose that we must receive something at least from someone every 30 sec ?
// If the Host is in stand-by mode, where it does not participate yet in any
// protocols... ?
// Could we merge that function with handleConn maybe ? so it timeout only on
// opened connections.
func (n *Host) Receive() network.ApplicationMessage {
	msgInfo := <-n.networkChan
	var data = msgInfo.Data
	if msgInfo.Data.MsgType == SDAMessageType {
		sda := data.Msg.(SDAMessage)
		t, msg, _ := network.UnmarshalRegisteredType(sda.DataSlice, data.Constructors)
		sda.MsgType = t
		sda.Data = msg
		data.Msg = sda
		dbg.Lvl3("SDA-Message is:", sda)
	}
	return data
}

// ProcessMessage checks if it is one of the messages for us or dispatch it
// to the corresponding instance.
// Our messages are:
// * SDAMessage - used to communicate between the Hosts
// * RequestTreeID - ask the parent for a given tree
// * SendTree - send the tree to the child
// * RequestPeerListID - ask the parent for a given peerList
// * SendPeerListID - send the tree to the child
func (n *Host) ProcessMessages() {
	for {
		msgInfo := <-n.networkChan
		dbg.Lvl3("Message Received:", msgInfo.Data)
		switch msgInfo.Data.MsgType {
		case SDAMessageType:
			n.processSDAMessage(msgInfo.Id, &msgInfo.Data)
		case RequestTreeType:
			tt := msgInfo.Data.Msg.(RequestTree).TreeID
			n.SendTo(msgInfo.Id, tt)
		case SendTreeType:
		case RequestIdentityListType:
		case SendIdentityListType:
		default:
			dbg.Error("Didn't recognize message", msgInfo.Data.MsgType)
		}
	}
}

// NetworkChan returns the channel where all messages received go. You can use
// it for testing, or if you want to make your own processing of the messages
func (n *Host) NetworkChan() chan MessageInfo {
	return n.networkChan
}

// AddProtocolInstance takes a UUID and a ProtocolInstance to be added
// to the map
func (n *Host) AddProtocolInstance(pi ProtocolInstance) {
	n.instances[pi.Id()] = pi
}

var timeOut = 30 * time.Second

// Handle a connection => giving messages to the MsgChans
func (n *Host) handleConn(id *Identity, address string, c network.Conn) {
	msgChan := make(chan network.ApplicationMessage)
	errorChan := make(chan error)
	doneChan := make(chan bool)
	go func() {
		for {
			select {
			case <-doneChan:
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
			n.networkChan <- MessageInfo{Id: id, Data: am}
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
func (n *Host) processSDAMessage(id *Identity, am *network.ApplicationMessage) error {
	sda := am.Msg.(SDAMessage)
	t, msg, err := network.UnmarshalRegisteredType(sda.DataSlice, network.DefaultConstructors(n.Suite()))
	if err != nil {
		dbg.Error("Error unmarshaling embedded msg in SDAMessage", err)
	}
	// Set the right type and msg
	sda.MsgType = t
	sda.Data = msg
	dbg.Lvl3("Processing SDA-message", sda)
	if !ProtocolExists(sda.ProtoID) {
		return fmt.Errorf("Protocol does not exists")
	}
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
func (n *Host) registerConnection(id *Identity, addr string, c network.Conn) {
	n.networkLock.Lock()
	n.identities[id.ID()] = id
	n.identityToAddress[id.ID()] = addr
	n.connections[addr] = c
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
	Id   *Identity
	Data network.ApplicationMessage
}

func init() {
	network.RegisterProtocolType(IdentityType, Identity{})
}

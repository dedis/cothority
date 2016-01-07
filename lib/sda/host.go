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
	"sync"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
	"time"
)

/*
Host is the structure responsible for holding information about the current
 state
*/
type Host struct {
	// Our address
	address string
	// The TCPHost
	host network.Host
	// The open connections
	connections map[string]network.Conn
	// and the locks
	networkLock *sync.Mutex
	// Our private-key
	private abstract.Secret
	// The suite used for this Host
	suite abstract.Suite
	// slice of received messages - testmode
	networkChan chan network.ApplicationMessage
	// instances linked to their ID and their ProtocolID
	instances map[UUID]ProtocolInstance
	// all trees known to this Host
	trees map[UUID]Tree
	// all identityLists known to this Host
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(address string, suite abstract.Suite, pkey abstract.Secret, host network.Host) *Host {
	n := &Host{
		networkLock: &sync.Mutex{},
		connections: make(map[string]network.Conn),
		address:     address,
		host:        host,
		private:     pkey,
		suite:       suite,
		networkChan: make(chan network.ApplicationMessage, 1),
		instances:   make(map[UUID]ProtocolInstance),
	}
	return n
}

// Start listening for messages coming from parent(up)
// each time a connection request is made, we receive first its identity then
// we handle the message using HandleConn
func (n *Host) Listen(address string) {
	fn := func(c network.Conn) {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		if err != nil || am.MsgType != IdentityMessageType {
			dbg.Lvl2(n.address, "Error receiving identity from connection", c.Remote())
		}
		id := am.Msg.(IdentityMessage)
		dbg.Lvl3(n.address, "Accepted Connection from", id.Name)
		n.networkLock.Lock()
		n.connections[address] = c
		n.networkLock.Unlock()

		n.handleConn(id.Name, c)
	}
	go n.host.Listen(address, fn)
}

// Connect takes an address where the next Host is
func (n *Host) Connect(address string) (network.Conn, error) {
	c, err := n.host.Open(address)
	if err != nil {
		return nil, err
	}
	id := IdentityMessage{n.address}
	if err := c.Send(context.TODO(), &id); err != nil {
		return nil, err
	}
	n.networkLock.Lock()
	n.connections[address] = c
	n.networkLock.Unlock()

	dbg.Lvl2("Host", n.address, "connected to", address)
	go n.handleConn(address, c)
	return c, nil
}

// Close shuts down the listener
func (n *Host) Close() error {
	return n.host.Close()
}

// SendTo takes the address of the remote peer to send a message to
func (n *Host) SendTo(address string, msg network.ProtocolMessage) error {
	n.networkLock.Lock()
	if c, ok := n.connections[address]; !ok {
		n.networkLock.Unlock()
		return fmt.Errorf("No connection to this address!", n.connections)
	} else {
		n.networkLock.Unlock()
		return c.Send(context.TODO(), msg)
	}
	return nil
}

// Receive will return the value of the communication-channel or an error
// if there has been nothing received during 2 seconds.
func (n *Host) Receive() (network.ApplicationMessage, error) {
	select {
	case data := <-n.networkChan:
		return data, nil
	case <-time.After(2 * time.Second):
		return network.ApplicationMessage{}, fmt.Errorf("Didn't receive in 2 seconds")
	}
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
		nm := <-n.networkChan
		fmt.Println("Message Received:", nm)
		switch nm.MsgType {
		case SDAMessageType:
			sda := nm.Msg.(SDAMessage)
			n.processSDAMessage(&sda)
			/*
				case RequestTreeType:
					tt := nm.Msg.(RequestTree).TreeID
					n.SendTo(nm.From, tt)
				case SendTreeType:
				case RequestIdentityListType:
				case SendIdentityListType:
			*/
		default:
			dbg.Error("Didn't recognize message", nm.MsgType)
		}
	}
}

// AddProtocolInstance takes a UUID and a ProtocolInstance to be added
// to the map
func (n *Host) AddProtocolInstance(pi ProtocolInstance) {
	n.instances[pi.Id()] = pi
}

// Handle a connection => giving messages to the MsgChans
func (n *Host) handleConn(address string, c network.Conn) {
	for {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		// So the receiver can know about the error
		am.SetError(err)
		am.From = address
		n.networkChan <- am
	}
}

// Dispatch SDA message looks if we have all the info to rightly dispatch the
// packet such as the protocol id and the topology id and the protocol instance
// id
func (n *Host) processSDAMessage(sda *SDAMessage) error {
	if !ProtocolExists(sda.ProtoID) {
		return fmt.Errorf("Protocol does not exists")
	}
	ip, ok := n.instances[sda.InstanceID]
	if !ok {
		// XXX What to do here ? create a new instance or just drop ?
		return fmt.Errorf("Instance Protocol not existing YET")
	}
	// Dispatch the message to the right instance !
	ip.Dispatch(sda)
	return nil
}

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
	"errors"
	"fmt"
	"sync"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

/*
Node is the structure responsible for holding information about the current
 state
*/
type Node struct {
	// The set of topologies that the protocols uses
	topologies map[TopologyID]Topology
	// instances linked to their ID and their ProtocolID
	instances map[ProtocolID]map[InstanceID]ProtocolInstance
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
	// The suite used for this node
	suite abstract.Suite
	// slice of received messages - testmode
	networkChan chan network.ApplicationMessage
}

// NewNode starts a new node that will listen on the network for incoming
// messages. It will store the private-key.
func NewNode(address string, suite abstract.Suite, pkey abstract.Secret, host network.Host) *Node {
	n := &Node{
		networkLock: &sync.Mutex{},
		connections: make(map[string]network.Conn),
		topologies:  make(map[TopologyID]Topology),
		address:     address,
		host:        host,
		private:     pkey,
		suite:       suite,
		networkChan: make(chan network.ApplicationMessage, 1),
		instances:   make(map[ProtocolID]map[InstanceID]ProtocolInstance),
	}
	return n
}

func (n *Node) Connect(address string) (network.Conn, error) {
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

	dbg.Lvl2("Node", n.address, "connected to", address)
	go n.handleConn(address, c)
	return c, nil
}

// Start listening for messages coming from parent(up)
// each time a connection request is made, we receive first its identity then
// we handle the message using HandleConn
func (n *Node) Listen(address string) {
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

// Handle a connection => giving messages to the MsgChans
func (n *Node) handleConn(address string, c network.Conn) {
	for {
		ctx := context.TODO()
		am, err := c.Receive(ctx)
		// So the receiver can know about the error
		am.SetError(err)
		am.From = address
		n.networkChan <- am
	}
}

// SendTo is the public method to send a message to someone using a given
// topology
func (n *Node) SendTo(topoId TopologyID, name string, data network.ProtocolMessage) {
	// Check the topology
	t, ok := n.topologies[topoId]
	// If none, return
	if !ok {
		fmt.Println("No topology for this id")
		return
	}

	// Check if we have the right to communicate with this peer
	// IN THIS TOPOLOGY
	if !t.IsConnectedTo(name) {
		fmt.Println("Can not communicate to this name")
		return
	}
	n.sendMessage(name, data)
}

// AddTopology is called by either the leader who created the topology
// or the peers contacted to run a protocol on this topology
func (n *Node) AddTopology(t Topology) {
	n.topologies[t.Id()] = t
}

func (n *Node) ProcessMessages() {
	for {
		nm := <-n.networkChan
		fmt.Println("Message Received:", nm)
		if nm.MsgType == SDAMessageType {
			sda := nm.Msg.(SDAMessage)
			n.processSDAMessage(&sda)
		}
	}
}

// ProtocolInfo is to be embedded in every message that is made for a
// ProtocolInstance
type SDAMessage struct {
	// The ID of the protocol
	ProtoID ProtocolID
	// The ID of the topology we use
	TopologyID TopologyID
	// The ID of the protocol instance - the counter
	InstanceID InstanceID
	// The ID of the peer in the protocol
	protoPeerID ProtocolPeerID

	// MsgType of the underlying data
	MsgType network.Type
	// The actual Data
	Data network.ProtocolMessage
}

// Dispatch SDA message looks if we have all the info to rightly dispatch the
// packet such as the protocol id and the topology id and the protocol instance
// id
func (n *Node) processSDAMessage(sda *SDAMessage) error {
	if !ProtocolExists(sda.ProtoID) {
		return fmt.Errorf("Protocol does not exists")
	}
	if _, ok := n.topologies[sda.TopologyID]; !ok {
		return fmt.Errorf("TopologyID does not exists")
	}
	var instances map[InstanceID]ProtocolInstance
	var ok bool
	if instances, ok = n.instances[sda.ProtoID]; !ok {
		return fmt.Errorf("Instances for this Protocol do not exist ")
	}
	var ip ProtocolInstance
	if ip, ok = instances[sda.InstanceID]; !ok {
		// XXX What to do here ? create a new instance or just drop ?
		return fmt.Errorf("Instance Protocol not existing YET")
	}
	// Dispatch the message to the right instance !
	ip.Dispatch(sda)
	return nil
}

// Add a protocolInstance to the list
func (n *Node) AddProtocolInstance(protoID ProtocolID, pi ProtocolInstance) {
	m, ok := n.instances[protoID]
	if !ok {
		m = make(map[InstanceID]ProtocolInstance)
		n.instances[protoID] = m
	}
	m[pi.Id()] = pi
}
func (n *Node) sendMessage(address string, msg network.ProtocolMessage) error {
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

// Close shuts down the listener
func (n *Node) Close() error {
	return n.host.Close()
}

// ProtocolInstanceConfig holds the configuration for one instance of the
// ProtocolInstance
// ?????????
type ProtocolInstanceConfig struct {
	IncomingPackets IPType
}

/*
IPType defines how incoming packets are handled
*/
type IPType int

const (
	WaitForAll IPType = iota
	PassDirect
	Timeout
)

// IdentityMessage used to notify a remote peer we want to connect to who we are
type IdentityMessage struct {
	Name string
}

func init() {
	network.RegisterProtocolType(IdentityMessageType, IdentityMessage{})
	network.RegisterProtocolType(SDAMessageType, SDAMessage{})
}

const (
	SDAMessageType = iota + 10
	IdentityMessageType
)

// NoSuchState indicates that the given state doesn't exist in the
// chosen ProtocolInstance
var NoSuchState error = errors.New("This state doesn't exist")

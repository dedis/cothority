/*
Implementation of the Secure Distributed API - main module

Node takes care about
* pre-parsing incoming packets
* setting up graphs and node-lists
* instantiating ProtocolInstances
* passing packets to ProtocolInstances
*/

package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
)

// NewNode starts a new node that will listen on the network for incoming
// messages. It will store the private-key.
func NewNode(address string, pkey abstract.Secret) *Node {
	n := &Node{
		address:     string,
		listener:    network.NewTcpHost(address),
		private:     pkey,
		suite:       edwards.NewAES128SHA256Ed25519(false),
		connections: make(map[string]network.Conn),
	}
	go n.listener.Listen(address, n.NewConnection)
	return n
}

/*
Node is the structure responsible for holding information about the current
 state
*/
type Node struct {
	// Our address
	address string
	// The TCPListener
	listener network.Host
	// The open connections
	connections map[string]network.Conn
	// Our private-key
	private abstract.Secret
	// The suite used for this node
	suite abstract.Suite
	// slice of received messages - testmode
	msgs []interface{}
}

// SendMessage sends a message
func (n *Node) SendMessage(t *TreePeer, msg interface{}) error {
	return nil
}

// TestSendMessage - send messages for testing
func (n *Node) TestSendMessage(n *Node, msg interface{}) error {

	return nil
}

// TestMessageRcv - waits for a message to be received
func (n *Node) TestMessageRcv() interface{} {
	return nil
}

// NewConnection handles a new connection-request.
func (n *Node) NewConnection(c network.Conn) {
	for {
		msg, err := c.Receive()
		n.connections[msg] = append(n.connections, c)
		if err != nil {
			dbg.Error("While receiving:", err)
		}
		dbg.Lvl1(msg)
	}
}

// ProtocolInstanceConfig holds the configuration for one instance of the
// ProtocolInstance
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

type Message struct {
	// The ID of the protocolInstance
	protocolID int
	// The ID representing the whole graph
	graphID int
	// The ID representing the node we received the message for
	graphNodeID int
	// The type of the message
	MessageType int
	// The message sent from the other peer
	Message interface{}
}

/*
Graph returns the corresponding graph
*/
func (m *Message) Graph() *TreePeer {
	return nil
}

/*
GraphNode returns the corresponding GraphNode
*/
func (m *Message) GraphNode() *TreePeer {
	return nil
}

// NoSuchState indicates that the given state doesn't exist in the
// chosen ProtocolInstance
var NoSuchState error = errors.New("This state doesn't exist")

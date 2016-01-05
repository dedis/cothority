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
)

// NewNode starts a new node that will listen on the network for incoming
// messages. It will store the private-key.
func NewNode(address string, pkey abstract.Secret) *Node {
	n := &Node{
		listener: network.NewTcpHost(address),
		private:  pkey,
	}
	n.listener.Listen(address, n.NewConnection)
	return n
}

/*
Node is the structure responsible for holding information about the current
 state
*/
type Node struct {
	// The TCPListener
	listener network.Host
	// The open connections
	connections []network.Conn
	// Our private-key
	private abstract.Secret
}

// SendMessage sends a message
func (n *Node) SendMessage(t *TreePeer, msg interface{}) error {
	return nil
}

// NewConnection handles a new connection-request.
func (n *Node) NewConnection(c network.Conn) {
	n.connections = append(n.connections, c)
	for {
		msg, err := c.Receive()
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

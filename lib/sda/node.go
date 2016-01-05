/*
Implementation of the Secure Distributed API - main module

Node takes care about
* pre-parsing incoming packets
* setting up graphs and node-lists
* instantiating ProtocolInstances
* passing packets to ProtocolInstances
*/

package sda

import "errors"

/*
NewNode starts a new node that will listen on the network for incoming
messages.
*/

func NewNode(listen string) *Node {
	return nil
}

/*
Node is the structure responsible for holding information about the current
 state
*/
type Node struct {
}

/*
SendMessage sends a message
*/
func (n *Node) SendMessage(gn *TreePeer, msg interface{}) error {
	return nil
}

/*
ProtocolInstanceConfig holds the configuration for one instance of the
 ProtocolInstance
*/
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

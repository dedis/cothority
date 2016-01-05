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
	"golang.org/x/net/context"
)

func init() {
	network.RegisterProtocolType(1, Message{})
}

// NewNode starts a new node that will listen on the network for incoming
// messages. It will store the private-key.
func NewNode(address string, pkey abstract.Secret) *Node {
	suite := edwards.NewAES128SHA256Ed25519(false)
	n := &Node{
		address:     address,
		host:        network.NewTcpHost(address, network.DefaultConstructors(suite)),
		private:     pkey,
		suite:       edwards.NewAES128SHA256Ed25519(false),
		connections: make(map[string]network.Conn),
		msgs:        make(chan interface{}, 1),
	}
	dbg.Print("Channel for", address, n, "is", n.msgs)
	err := n.host.Listen(address, n.NewConnection)
	if err != nil {
		dbg.Error("Couldn't open", address, "for listening:", err)
	}
	return n
}

/*
Node is the structure responsible for holding information about the current
 state
*/
type Node struct {
	// Our address
	address string
	// The TCPHost
	host network.Host
	// The open connections
	connections map[string]network.Conn
	// Our private-key
	private abstract.Secret
	// The suite used for this node
	suite abstract.Suite
	// slice of received messages - testmode
	msgs chan interface{}
}

// SendMessage sends a message
func (n *Node) SendMessage(t *TreePeer, msg interface{}) error {
	return nil
}

// Close shuts down the listener
func (n *Node) Close() error {
	return n.host.Close()
}

// TestSendMessage - send messages for testing
func (n *Node) TestSendMessage(dest *Node, msg interface{}) error {
	c, ok := n.connections[dest.address]
	if !ok {
		dbg.Lvl3("Creating connection to", dest.address)
		var err error
		c, err = n.host.Open(dest.address)
		if err != nil {
			return err
		}
		n.connections[dest.address] = c
	}
	msg_send := &Message{
		Message: msg,
	}
	dbg.Lvl3("Sending message", msg_send)
	// TODO: use msg_send as the message to send
	return c.Send(context.TODO(), msg)
}

// TestMessageRcv - waits for a message to be received
func (n *Node) TestMessageRcv() interface{} {
	dbg.Print("Listening for message in", n.address)
	dbg.Print("Rcv-Channel is", n.msgs)
	msg := <-n.msgs
	dbg.Print("Got message", msg)
	return msg
}

// NewConnection handles a new connection-request.
func (n *Node) NewConnection(c network.Conn) {
	dbg.Lvl3("Getting new connection from", c, n)
	for {
		msg, err := c.Receive(context.TODO())
		dbg.Lvl3(n.address, "received message", msg, "from", msg.From)
		n.connections[msg.From] = c
		if err != nil {
			dbg.Error("While receiving:", err)
		}
		dbg.Print("Send-Channel is", n.msgs)
		n.msgs <- msg.Msg
		dbg.Lvl1(msg, "in", n.address)
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

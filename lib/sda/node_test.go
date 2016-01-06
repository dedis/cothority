// better not get sda_test, cannot access unexported fields
package sda

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
	"testing"
	"time"
)

var suite abstract.Suite = edwards.NewAES128SHA256Ed25519(false)

func init() {
	network.RegisterProtocolType(SimpleMessageType, SimpleMessage{})
}

// Test setting up of Node
func TestNodeNew(t *testing.T) {
	node1 := newNode("localhost:2000", suite)
	if node1 == nil {
		t.Fatal("Couldn't setup a node")
	}
	err := node1.Close()
	if err != nil {
		t.Fatal("Couldn't close", err)
	}
}

// Test closing and opening of Node on same address
func TestNodeClose(t *testing.T) {
	node1 := newNode("localhost:2000", suite)
	err := node1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	node1 = newNode("localhost:2000", suite)
	// Needs to wait some as the listener will try for a couple of times
	// to make the connection
	time.Sleep(time.Second)
	node1.Close()
}

// Test connection of multiple Nodes and sending messages back and forth
func TestNodeMessaging(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	msg := &SimpleMessage{3}
	node1 := newNode("localhost:2000", suite)
	// make the second peer as the server
	node2 := newNode("localhost:2001", suite)
	// make it listen
	node2.Listen("localhost:2001")
	_, err := node1.Connect("localhost:2001")
	if err != nil {
		t.Fatal(err)
	}
	err = node1.sendMessage("localhost:2001", msg)
	if err != nil {
		t.Fatal("Couldn't send from node2 -> node1:", err)
	}
	var msgDec SimpleMessage
	select {
	case data := <-node2.networkChan:
		var ok bool
		if data.MsgType != SimpleMessageType {
			t.Fatal("Did not receive the expected type")
		}
		if msgDec, ok = data.Msg.(SimpleMessage); !ok {
			t.Fatal("Can not convert the message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Did not receive message ..")
	}
	if msgDec.I != 3 {
		t.Fatal("Received message from node2 -> node1 is wrong")
	}
}

type SimpleMessage struct {
	I int
}

const (
	SimpleMessageType = iota + 50
)

// Test parsing of incoming packets with regard to its double-included
// data-structure

// Test propagation of peer-lists - both known and unknown
func TestPeerListPropagation(t *testing.T) {

}

// Test propagation of graphs - both known and unknown

// Test instantiation of ProtocolInstances

// Test access of actual peer that received the message
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph - ProtocolID + GraphID + InstanceID is not enough
// XXX ???

// Test complete parsing of new incoming packet
// - Test if it is SDAMessage
// - reject if unknown ProtocolID
// - setting up of graph and nodelist
// - instantiating ProtocolInstance

// privPub creates a private/public key pair
func privPub(s abstract.Suite) (abstract.Secret, abstract.Point) {
	keypair := &config.KeyPair{}
	keypair.Gen(s, random.Stream)
	return keypair.Secret, keypair.Public
}

func newNode(address string, s abstract.Suite) *Node {
	priv, _ := privPub(s)
	return NewNode(address, s, priv, network.NewTcpHost(network.DefaultConstructors(s)))
}

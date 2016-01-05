package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
	"testing"
	"time"
)

var suite abstract.Suite = edwards.NewAES128SHA256Ed25519(false)

func init() {
	network.RegisterProtocolType(1, SimpleMessage{})
}

// Test setting up of Node
func TestNewNode(t *testing.T) {
	priv1, _ := privPub(suite)
	node1 := sda.NewNode("localhost:2000", priv1)
	if node1 == nil {
		t.Fatal("Couldn't setup a node")
	}
	err := node1.Close()
	if err != nil {
		t.Fatal("Couldn't close", err)
	}
}

// Test closing and opening of Node on same address
func TestClose(t *testing.T) {
	priv1, _ := privPub(suite)
	node1 := sda.NewNode("localhost:2000", priv1)
	err := node1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	node1 = sda.NewNode("localhost:2000", priv1)
	// Needs to wait some as the listener will try for a couple of times
	// to make the connection
	time.Sleep(time.Second)
	node1.Close()
}

// Test connecting of multiple Nodes
func TestNewNodes(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	msg := &SimpleMessage{3}
	priv1, _ := privPub(suite)
	node1 := sda.NewNode("localhost:2000", priv1)
	priv2, _ := privPub(suite)
	node2 := sda.NewNode("localhost:2001", priv2)
	err := node2.TestSendMessage(node1, msg)
	if err != nil {
		t.Fatal("Couldn't send from node2 -> node1:", err)
	}
	time.Sleep(time.Second)
	msg_rcv := node1.TestMessageRcv().(SimpleMessage)
	if msg_rcv.I != 3 {
		t.Fatal("Received message from node2 -> node1 is wrong")
	}
	msg.I = 5
	err = node1.TestSendMessage(node2, msg)
	if err != nil {
		t.Fatal("Couldn't send from node2 -> node1:", err)
	}
	msg_rcv = node2.TestMessageRcv().(SimpleMessage)
	if msg_rcv.I != 5 {
		t.Fatal("Received message from node1 -> node2 is wrong")
	}
}

type SimpleMessage struct {
	I int
}

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

// Test complete parsing of new incoming packet
// - reject if unknown ProtocolID
// - setting up of graph and nodelist
// - instantiating ProtocolInstance

// privPub creates a private/public key pair
func privPub(s abstract.Suite) (abstract.Secret, abstract.Point) {
	keypair := &config.KeyPair{}
	keypair.Gen(s, random.Stream)
	return keypair.Secret, keypair.Public
}

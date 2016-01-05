package sda_test

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
	"testing"
)

var suite abstract.Suite = edwards.NewAES128SHA256Ed25519(false)

// Test setting up of Node
func TestNewNode(t *testing.T) {
	priv1, _ := privPub(suite)
	node1 := sda.NewNode("localhost:2000", priv1)
	if node1 == nil {
		t.Fatal("Couldn't setup a node")
	}
}

// Test connecting of multiple Nodes
func TestNewNodes(t *testing.T) {
	msg := struct{ i int }{3}
	priv1, _ := privPub(suite)
	node1 := sda.NewNode("localhost:2000", priv1)
	priv2, _ := privPub(suite)
	node2 := sda.NewNode("localhost:2001", priv2)
	node2.TestSendMessage(node1, msg)
	node1.TestSendMessage(node2, msg)
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

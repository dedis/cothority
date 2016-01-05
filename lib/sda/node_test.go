package sda_test

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
	"testing"
)

var suite abstract.Suite = edwards.NewAES128SHA256Ed25519(false)

// Test setting up of Node
func TestNewNode(t *testing.T) {
	priv, pub := privPub(suite)

}

// Test connecting of multiple Nodes

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

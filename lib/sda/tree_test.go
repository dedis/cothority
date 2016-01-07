package sda_test

import (
	"strconv"
	"testing"

	"github.com/dedis/crypto/edwards/ed25519"
)

var tSuite = ed25519.NewAES128SHA256Ed25519(false)

// genLocalhostPeerNames will generate n localhost names with port indices starting from p
func genLocalhostPeerNames(n, p int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = "localhost:" + strconv.Itoa(p+i)
	}
	return names
}

// TODO  test the ID generation
func TestTopologyId(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	peerList := GenPeerList(tSuite, names)
	// Generate two example topology
	root, _ := ExampleGenerateTreeFromPeerList(peerList)
	t := Topology{PeerList: peerList, TreeNode: root}
	genId := string(peerList.Id()) + root.Id()
	if genId != t.Id() {
		t.Fatal("Id generated is wrong")
	}
}

// Test if topology correctly handles the "virtual" connections in the topology
func TestTreeConnectedTo(t *testing.T) {

	names := genLocalhostPeerNames(3, 2000)
	peerList := GenPeerList(tSuite, names)
	// Generate two example topology
	root, _ := ExampleGenerateTreeFromPeerList(peerList)
	// Generate the network
	if !root.IsConnectedTo("localhost:2001") {
		t.Fatal("Root should be connection to localhost:2001")
	}

}

func ExampleGenerateTreeFromPeerList(pl *PeerList) (*TreeNode, []*TreeNode) {
	var nodes []*TreeNode
	var leaderId int
	for n, _ := range pl.Peers {
		nodes = append(nodes, NewTree(n))
		if n == "localhost:1000" {
			leaderId = len(nodes) - 1
		}
	}
	// Very simplistic depth-2 tree
	for i := 0; i < len(nodes); i++ {
		if i == leaderId {
			continue
		}
		nodes[leaderId].AddChild(nodes[i])
	}
	return nodes[leaderId], nodes
}

// Test initialisation of new peer-list
func TestInitPeerList(t *testing.T) {
	adresses := []string{"localhost:1010", "localhost:1012"}
	pl := sda.GenPeerList(tSuite, adresses)
	if pl == nil {
		t.Fatal("Could not initialize PeerList")
	}
	if len(pl.Peers) != 2 {
		t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.Peers))
	}
	if pl.Id == nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(pl.Id) != sha256.Size {
		t.Fatal("PeerList ID does not seem to be a sha256 hash.")
	}
}

// Test initialisation of new peer-list from config-file
func TestInitPeerListFromConfigFile(t *testing.T) {
	pl := sda.PeerList{}
	app.ReadTomlConfig(&pl, "2_peers.toml", "testdata")
	if len(pl.Peers) != 2 {
		t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.Peers))
	}
	// XXX do we want the ID to be regenerated, or do we want to read it from file, or both cases?
	// this test should define what behaviour is to expect ...
	if pl.Id == nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(pl.Id) != sha256.Size {
		t.Fatal("PeerList ID does not seem to be a sha256 hash.")
	}
	pl2 := sda.PeerList{}
	app.ReadTomlConfig(&pl2, "3_peers.toml", "testdata")
	if len(pl2.Peers) != 3 {
		t.Fatalf("Expected 3 peers in PeerList. Instead got %d", len(pl.Peers))
	}
	if pl.Id == nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(pl.Id) != sha256.Size {
		t.Fatal("PeerList ID does not seem to be a sha256 hash.")
	}
}

// Test initialisation of new random tree from a peer-list

// Test initialisation of new graph from config-file using a peer-list
// XXX again this test might be obsolete/does more harm then it is useful:
// It forces every field to be exported/made public
// and we want to get away from config files (or not?)

// Test initialisation of new graph when one peer is represented more than
// once

// Test access to graph:
// - parent
// - children
// - public keys
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph

// genLocalhostPeerNames will generate n localhost names with port indices starting from p
func genLocalhostPeerNames(n, p int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = "localhost" + strconv.Itoa(p+i)
	}
	return names
}

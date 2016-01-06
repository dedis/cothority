package sda

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

}

// Test if topology correctly handles the "virtual" connections in the topology
func TestTopologyConnectedTo(t *testing.T) {

	names := genLocalhostPeerNames(3, 2000)
	peerList := GenPeerList(tSuite, names)
	// Generate two example topology
	root, _ := ExampleGenerateTreeFromPeerList(peerList)
	graph := ExampleGenerateGraphFromPeerList(peerList)
	// Generate the network
	if !root.IsConnectedTo("localhost:2001") {
		t.Fatal("Root should be connection to localhost:2001")
	}
	if !graph.IsConnectedTo("localhost:2002") {
		t.Fatal("Graph root should be connected to localhost:2002")
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

func ExampleGenerateGraphFromPeerList(pl *PeerList) *GraphNode {
	leader := NewGraph("localhost:1000")
	for n, _ := range pl.Peers {
		if n != "localhost:1010" {
			leader.AddEdge(n)
		}
	}
	return leader
}

package sda

import (
	"fmt"
	"strconv"
	"testing"

	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/network"
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

// Mock connection
type TestConnection struct {
	Name string
}

func (c TestConnection) Close() error {
	// for tests only
	return nil
}

func (c TestConnection) Send(ctx context.Context, data network.ProtocolMessage) error {
	fmt.Println("Sent data to connection", c.Name)
	return nil
}

func (c TestConnection) Remote() string {
	return c.Name
}

func (c TestConnection) Receive(ctx context.Context) (network.ApplicationMessage, error) {
	// for tests only:
	fmt.Println("Received")
	return network.ApplicationMessage{}, nil
}

func TestTopology(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	peerList := GenPeerList(tSuite, names)
	// Generate two example topology
	root, _ := ExampleGenerateTreeFromPeerList(peerList)
	graph := ExampleGenerateGraphFromPeerList(peerList)
	// Generate the network
	// We are the leader / root
	node := NewNode("localhost:1010", nil)
	// These are like physical connections through the network
	node.AddConnection(TestConnection{"localhost:2000"})
	node.AddConnection(TestConnection{"localhost:2001"})
	node.AddConnection(TestConnection{"localhost:2002"})

	// Let's say we use two different topology at the same time
	node.AddTopology(graph)
	node.AddTopology(root)

	// Then send to two nodes, using one particular topology
	node.SendTo(root.Id(), "localhost:2000", []byte("Message-for-you-node-2"))
	node.SendTo(graph.Id(), "localhost:2001", []byte("Message-for-you-node-2"))
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

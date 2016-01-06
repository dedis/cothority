// topology is a general
package sda

import (
	"bytes"
	"fmt"

	"github.com/dedis/cothority/lib/network"

	"golang.org/x/net/context"
)

type ID string

func (n *Node) SendTo(topoId ID, name string, data []byte) {
	// Check the topology
	t, ok := n.topologies[topoId]
	// If none, return
	if !ok {
		fmt.Println("No topology for this id")
		return
	}

	// Check if we have the right to communicate with this peer
	// IN THIS TOPOLOGY
	if !t.IsConnectedTo(name) {
		fmt.Println("Can not communicate to this name")
		return
	}
	// Check if we have the connection to that node
	if c, ok := n.connections[name]; !ok {
		fmt.Println("No network for this name")
		return
	} else {
		c.Send(context.TODO(), data)
	}
}

func (n *Node) AddTopology(t Topology) {
	n.topologies[t.Id()] = t
}

func (n *Node) AddConnection(c network.Conn) {
	n.connections[c.Remote()] = c
}

// a generic topology to be used by any network layer/host layer
// it just determines, if we can comunicate with a given peer or not
type Topology interface {
	Id() ID
	IsConnectedTo(name string) bool
}

// Graph implementation
type GraphNode struct {
	// name of the vertice we are on
	Name      string
	adjacency map[string]int
}

// sample ID function (have to take into account adjacency matrix)
func (g *GraphNode) Id() ID {
	return ID(g.Name)
}
func (g *GraphNode) IsConnectedTo(name string) bool {
	if _, ok := g.adjacency[name]; !ok {
		return false
	}
	return true
}

func (g *GraphNode) AddEdge(vertice2 string) {
	g.adjacency[vertice2] = 1
}

func NewGraph(name string) *GraphNode {
	adj := make(map[string]int)
	return &GraphNode{
		Name:      name,
		adjacency: adj,
	}
}

func GenerateGraph(base string, n int) *GraphNode {
	// let's simulate only the point of view of ONE vertice in the graph
	g := NewGraph("vertice")
	for i := 0; i < n; i++ {
		g.AddEdge(fmt.Sprintf("%s-%d", base, i))
	}
	return g
}

// TREE Implementation
type TreeNode struct {
	Name     string
	Parent   *TreeNode `protobuf:"-"`
	Children []*TreeNode
}

func (t *TreeNode) Id() ID {
	var buf bytes.Buffer
	if t.Parent != nil {
		buf.Write([]byte(t.Parent.Name))
	}
	buf.Write([]byte(t.Name))
	for i := range t.Children {
		buf.Write([]byte(t.Children[i].Name))
	}
	return ID(buf.String())
}

// Check if it can communicate with parent or children
func (t *TreeNode) IsConnectedTo(name string) bool {
	if t.Parent != nil && t.Parent.Name == name {
		return true
	}

	for i := range t.Children {
		if t.Children[i].Name == name {
			return true
		}
	}
	return false
}

func (t *TreeNode) AddChild(c *TreeNode) {
	t.Children = append(t.Children, c)
}

func NewTree(name string) *TreeNode {
	return &TreeNode{
		Name:     name,
		Parent:   nil,
		Children: make([]*TreeNode, 0),
	}
}

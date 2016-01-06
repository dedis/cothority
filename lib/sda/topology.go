// topology is a general
package sda

import (
	"bytes"
	"fmt"
	"github.com/dedis/cothority/lib/network"
)

func init() {
	network.RegisterProtocolType(GraphType, Graph{})
	network.RegisterProtocolType(TreeNodeType, TreeNode{})
}

type TopologyID string

// a generic topology to be used by any network layer/host layer
// it just determines, if we can comunicate with a given peer or not
type Topology interface {
	Id() TopologyID
	IsConnectedTo(name string) bool
}

// Graph implementation
type GraphNode struct {
	// name of the vertice we are on
	Name      string
	adjacency map[string]int
}

// sample ID function (have to take into account adjacency matrix)
func (g *GraphNode) Id() TopologyID {
	return TopologyID(g.Name)
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
	Name string
	// parent *TreeNode `protobuf:"-"`would be ideal because if you serialize
	// this with protobuf, it makes a veryyy big message because of the
	// recursion in the parent's parent etc. but not implemented for now in
	// protobuf so we pass only the local sub tree to each peer
	Parent   string
	Children []*TreeNode
}

func (t *TreeNode) Id() TopologyID {
	var buf bytes.Buffer
	if t.Parent != "" {
		buf.Write([]byte(t.Parent))
	}
	buf.Write([]byte(t.Name))
	for i := range t.Children {
		buf.Write([]byte(t.Children[i].Name))
	}
	return TopologyID(buf.String())
}

// Check if it can communicate with parent or children
func (t *TreeNode) IsConnectedTo(name string) bool {
	if t.Parent == name {
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
		Parent:   "",
		Children: make([]*TreeNode, 0),
	}
}

const (
	GraphType = iota + 100
	TreeNodeType
)

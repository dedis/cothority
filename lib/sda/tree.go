// topology is a general
package sda

import (
	"bytes"
	"fmt"
	"github.com/dedis/cothority/lib/network"
)

// In this file we define the main structures used for a running protocol
// instance. First there is the Identity struct: it represents the Identity a
// someone, a server over the internet, mainly tighted by its public key.
// Then the Topology, which contains everything a
// The tree contains the peerId which is the ID given to a an Identity / server
// during one protocol instance. A server can have many peerId in one tree.
// ProtocolInstance needs to know that means :
// - which IdentityList we are using ( a selection of proper servers )
// - which Tree we are using.
// - The overlay network : a mapping from PeerId
// It contains the PeerId of the parent and the sub tree of the children.
func init() {
	network.RegisterProtocolType(GraphType, Graph{})
	network.RegisterProtocolType(TreeNodeType, TreeNode{})
}

// An Identity is used to represent a SERVER / PEER in the whole intenet
// its main identity is its public key, then we get some means, some address on
// where to contact him.
type Identity struct {
	Public    abstract.Point
	Addresses []string
}

// Mapping is used to relate from an Identity to a PeerId in the protocol
// instance
type Mapping struct {
	Identity
	PeerId
}

type TreeID string

// Overlay network is a serie of ampping between identities and protocol
// instance.
type Overlay []Mapping

// a topology to be used by any network layer/host layer
// It contains the peer list we use, and the tree we use
type Tree struct {
	IdList *IdentityList
	Root   *TreeNode
}

func (t *Tree) Id() TreeID {
	return string(t.PeerList.Id()) + t.TreeNode.Id()
}

// A PeerList is a list of Identity we choose to run  some tree on it ( and
// therefor some protocols)
type IdentityList struct {
	ID   string
	List []Identity
}

func (pl *IdentityList) Id() string {
	if pl.ID == "" {
		pl.generateId()
	}
	return pl.ID
}

func (pl *IdentityList) generateId() {
	var buf bytes.Buffer
	for _, n := range pl.Nodes {
		b, _ := n.Public.MarshalBinary()
		buf.Write(b)
	}
	pl.ID = buf.String()
}

// TREE Implementation
type TreeNode struct {
	// The peerID is the ID of a server / node, FOR THIS PROTOCOL
	// a server can have many peerId during one protocol instance
	PeerId string
	nodeId Identity
	// parent *TreeNode `protobuf:"-"`would be ideal because if you serialize
	// this with protobuf, it makes a veryyy big message because of the
	// recursion in the parent's parent etc. but not implemented for now in
	// protobuf so we pass only the local sub tree to each peer
	Parent   string
	Children []*TreeNode
}

func (t *TreeNode) Id() string {
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
	TopologyType = iota + 10
	TreeNodeType
)

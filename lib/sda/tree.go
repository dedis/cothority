// topology is a general
package sda

import (
	"bytes"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
)

// In this file we define the main structures used for a running protocol
// instance. First there is the Identity struct: it represents the Identity of
// someone, a server over the internet, mainly tied by its public key.
// The tree contains the peerId which is the ID given to a an Identity / server
// during one protocol instance. A server can have many peerId in one tree.
// ProtocolInstance needs to know:
// - which IdentityList we are using ( a selection of proper servers )
// - which Tree we are using.
// - The overlay network: a mapping from PeerId
// It contains the PeerId of the parent and the sub tree of the children.
func init() {
	network.RegisterProtocolType(TreeNodeType, TreeNode{})
}

type UUID string

// An Identity is used to represent a SERVER / PEER in the whole internet
// its main identity is its public key, then we get some means, some address on
// where to contact him.
type Identity struct {
	Public    abstract.Point
	Addresses []string
}

func NewIdentity(public abstract.Point, addresses ...string) {
	return &Identity{
		Public:    public,
		Addresses: addresses,
	}
}

type TreeID string

// a topology to be used by any network layer/host layer
// It contains the peer list we use, and the tree we use
type Tree struct {
	IdList *IdentityList
	Root   *TreeNode
}

func (t *Tree) Id() TreeID {
	return TreeID(string(t.IdList.Id()) + t.Root.Id())
}

// A PeerList is a list of Identity we choose to run  some tree on it ( and
// therefor some protocols)
type IdentityList struct {
	ID   string
	List []*Identity
}

func NewIdentityList(ids []*Identity) IdentityList {
	return IdentityList{List: ids}
}

func (pl *IdentityList) Id() string {
	if pl.ID == "" {
		pl.generateId()
	}
	return pl.ID
}

func (pl *IdentityList) generateId() {
	var buf bytes.Buffer
	for _, n := range pl.List {
		b, _ := n.Public.MarshalBinary()
		buf.Write(b)
	}
	pl.ID = buf.String()
}

// TreeNode is one node in the tree
type TreeNode struct {
	// The peerID is the ID of a server / node, FOR THIS PROTOCOL
	// a server can have many peerId during one protocol instance
	PeerId string
	NodeId *Identity
	// parent *TreeNode `protobuf:"-"`would be ideal because if you serialize
	// this with protobuf, it makes a very big message because of the
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
	buf.Write([]byte(t.PeerId))
	for i := range t.Children {
		buf.Write([]byte(t.Children[i].PeerId))
	}
	return buf.String()
}

// Check if it can communicate with parent or children
func (t *TreeNode) IsConnectedTo(name string) bool {
	if t.Parent == name {
		return true
	}

	for i := range t.Children {
		if t.Children[i].PeerId == name {
			return true
		}
	}
	return false
}

func (t *TreeNode) AddChild(c *TreeNode) {
	t.Children = append(t.Children, c)
}

func NewTreeNode(name string, ni *Identity) *TreeNode {
	return &TreeNode{
		PeerId:   name,
		NodeId:   ni,
		Parent:   "",
		Children: make([]*TreeNode, 0),
	}
}

const (
	TopologyType = iota + 10
	TreeNodeType
)

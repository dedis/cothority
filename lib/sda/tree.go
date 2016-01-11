// topology is a general
package sda

import (
	"bytes"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	. "github.com/satori/go.uuid"
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

// a topology to be used by any network layer/host layer
// It contains the peer list we use, and the tree we use
type Tree struct {
	IdList *IdentityList
	Root   *TreeNode
}

func (t *Tree) Id() UUID {
	var h bytes.Buffer
	h.Write(t.IdList.Id().Bytes())
	h.Write(t.Root.Id().Bytes())
	u := NewV5(NamespaceURL, h.String())
	return u
}

// A PeerList is a list of Identity we choose to run  some tree on it ( and
// therefor some protocols)
type IdentityList struct {
	ID   UUID
	List []*network.Identity
}

func NewIdentityList(ids []*network.Identity) *IdentityList {
	return &IdentityList{List: ids}
}

func (pl *IdentityList) Id() UUID {
	if pl.ID == Nil {
		pl.generateId()
	}
	return pl.ID
}

func (pl *IdentityList) generateId() {
	var h bytes.Buffer
	for i := range pl.List {
		b, _ := pl.List[i].Public.MarshalBinary()
		h.Write(b)
	}
	u, err := FromBytes(h.Bytes()[0:16])
	if err != nil {
		panic(err)
	}
	pl.ID = u
}

// TreeNode is one node in the tree
type TreeNode struct {
	// The peerID is the ID of a server / node, FOR THIS PROTOCOL
	// a server can have many peerId during one protocol instance
	PeerId string
	NodeId *network.Identity
	// parent *TreeNode `protobuf:"-"`would be ideal because if you serialize
	// this with protobuf, it makes a very big message because of the
	// recursion in the parent's parent etc. but not implemented for now in
	// protobuf so we pass only the local sub tree to each peer
	Parent   string
	Children []*TreeNode
}

func (t *TreeNode) Id() UUID {
	var buf bytes.Buffer
	if t.Parent != "" {
		buf.Write([]byte(t.Parent))
	}
	buf.Write([]byte(t.PeerId))
	for i := range t.Children {
		buf.Write([]byte(t.Children[i].PeerId))
	}
	u := NewV5(NamespaceURL, buf.String())
	return u
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

func NewTreeNode(name string, ni *network.Identity) *TreeNode {
	return &TreeNode{
		PeerId:   name,
		NodeId:   ni,
		Parent:   "",
		Children: make([]*TreeNode, 0),
	}
}
func (t *TreeNode) String() string {
	return t.PeerId
}
func (t *TreeNode) Stringify() string {
	var buf bytes.Buffer
	var lastDepth int
	fn := func(d int, n *TreeNode) {
		if d > lastDepth {
			buf.Write([]byte("\n\n"))
		} else {
			buf.Write([]byte(n.PeerId))
		}
	}
	t.Visit(0, fn)
	return buf.String()
}

func (t *TreeNode) Visit(firstDepth int, fn func(depth int, n *TreeNode)) {
	fn(firstDepth, t)
	for i := range t.Children {
		t.Children[i].Visit(firstDepth+1, fn)
	}
}

// IdentityListToml is the struct can can embbed IdentityToml to be written in a
// toml file
type IdentityListToml struct {
	ID   UUID
	List []*network.IdentityToml
}

// Toml returns the toml-writtable version of this identityList
func (id *IdentityList) Toml(suite abstract.Suite) *IdentityListToml {
	ids := make([]*network.IdentityToml, len(id.List))
	for i := range id.List {
		ids[i] = id.List[i].Toml(suite)
	}
	return &IdentityListToml{
		ID:   id.ID,
		List: ids,
	}
}

// IdentityList returns the Id list from this toml read struct
func (id *IdentityListToml) IdentityList(suite abstract.Suite) *IdentityList {
	ids := make([]*network.Identity, len(id.List))
	for i := range id.List {
		ids[i] = id.List[i].Identity(suite)
	}
	return &IdentityList{
		ID:   id.ID,
		List: ids,
	}
}

const (
	TopologyType = iota + 10
	TreeNodeType
	IdentityType
)

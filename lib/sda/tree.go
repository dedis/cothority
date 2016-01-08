// topology is a general
package sda

import (
	"bytes"
	"crypto/sha256"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"hash"
	"strings"
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
	network.RegisterProtocolType(IdentityType, Identity{})
}

// Universal Uniquely Identifier
type UUID string

// XXX TMp solution of hashing identifier so we have a UUID
var NewHashFunc func() hash.Hash = sha256.New

// An Identity is used to represent a SERVER / PEER in the whole internet
// its main identity is its public key, then we get some means, some address on
// where to contact him.
type Identity struct {
	Public    abstract.Point
	Addresses []string
	iter      int
}

// First returns the first address available
func (id *Identity) First() string {
	if len(id.Addresses) > 0 {
		return id.Addresses[0]
	}
	return ""
}

// Next returns the next address like an iterator
func (id *Identity) Next() string {
	if len(id.Addresses) < id.iter+1 {
		return ""
	}
	addr := id.Addresses[id.iter]
	id.iter++
	return addr

}

func (id *Identity) ID() UUID {
	h := NewHashFunc()
	buf, _ := id.Public.MarshalBinary()
	h.Write(buf)
	return UUID(h.Sum(nil))
}
func NewIdentity(public abstract.Point, addresses ...string) *Identity {
	return &Identity{
		Public:    public,
		Addresses: addresses,
	}
}

// a topology to be used by any network layer/host layer
// It contains the peer list we use, and the tree we use
type Tree struct {
	IdList *IdentityList
	Root   *TreeNode
}

func (t *Tree) Id() UUID {
	h := NewHashFunc()
	h.Write([]byte(t.IdList.Id()))
	h.Write([]byte(t.Root.Id()))
	return UUID(h.Sum(nil))
}

// A PeerList is a list of Identity we choose to run  some tree on it ( and
// therefor some protocols)
type IdentityList struct {
	ID   UUID
	List []*Identity
}

func NewIdentityList(ids []*Identity) *IdentityList {
	return &IdentityList{List: ids}
}

func (pl *IdentityList) Id() UUID {
	if pl.ID == "" {
		pl.generateId()
	}
	return pl.ID
}

func (pl *IdentityList) generateId() {
	h := NewHashFunc()
	for i := range pl.List {
		b, _ := pl.List[i].Public.MarshalBinary()
		h.Write(b)
	}
	pl.ID = UUID(h.Sum(nil))
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

func (t *TreeNode) Id() UUID {
	buf := NewHashFunc()
	if t.Parent != "" {
		buf.Write([]byte(t.Parent))
	}
	buf.Write([]byte(t.PeerId))
	for i := range t.Children {
		buf.Write([]byte(t.Children[i].PeerId))
	}
	return UUID(buf.Sum(nil))
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

// IdentityToml is the struct that can be marshalled into a toml file
type IdentityToml struct {
	Public    string
	Addresses []string
}

// IdentityListToml is the struct can can embbed IdentityToml to be written in a
// toml file
type IdentityListToml struct {
	ID   UUID
	List []*IdentityToml
}

func (id *Identity) Toml(suite abstract.Suite) *IdentityToml {
	var buf bytes.Buffer
	cliutils.WritePub64(suite, &buf, id.Public)
	return &IdentityToml{
		Addresses: id.Addresses,
		Public:    buf.String(),
	}
}

func (id *IdentityList) Toml(suite abstract.Suite) *IdentityListToml {
	ids := make([]*IdentityToml, len(id.List))
	for i := range id.List {
		ids[i] = id.List[i].Toml(suite)
	}
	return &IdentityListToml{
		ID:   id.ID,
		List: ids,
	}
}

func (id *IdentityToml) Identity(suite abstract.Suite) *Identity {
	pub, _ := cliutils.ReadPub64(suite, strings.NewReader(id.Public))
	return &Identity{
		Public:    pub,
		Addresses: id.Addresses,
	}
}

func (id *IdentityListToml) IdentityList(suite abstract.Suite) *IdentityList {
	ids := make([]*Identity, len(id.List))
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

// topology is a general
package sda

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/satori/go.uuid"
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
	network.RegisterProtocolType(TreeType, Tree{})
	network.RegisterProtocolType(TreeMarshalType, TreeMarshal{})
	network.RegisterProtocolType(TreeNodeType, TreeNode{})
	network.RegisterProtocolType(IdentityType, Identity{})
	network.RegisterProtocolType(IdentityListType, Identity{})
}

// XXX TMp solution of hashing identifier so we have a UUID
var NewHashFunc func() hash.Hash = sha256.New

// An Identity is used to represent a SERVER / PEER in the whole internet
// its main identity is its public key, then we get some means, some address on
// where to contact him.
type Identity struct {
	// This is the public key of that identity
	Public abstract.Point
	// The UUID corresponding to that public key
	Id uuid.UUID
	// A slice of addresses of where that Id might be found
	Addresses []string
	// used to return the next available address
	iter int
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

// NewIdentity creates a new identity based on a public key and with a slice
// of IP-addresses where to find that identity. The Id is based on a
// version5-UUID which can include a URL that is based on it's public key.
func NewIdentity(public abstract.Point, addresses ...string) *Identity {
	url := "https://dedis.epfl.ch/id/" + public.String()
	return &Identity{
		Public:    public,
		Addresses: addresses,
		Id:        uuid.NewV5(uuid.NamespaceURL, url),
	}
}

// Tree is a topology to be used by any network layer/host layer
// It contains the peer list we use, and the tree we use
type Tree struct {
	Id     uuid.UUID
	IdList *IdentityList
	Root   *TreeNode
}

// NewTree creates a new tree using the identityList and the root-node. It
// also generates the id.
func NewTree(il *IdentityList, r *TreeNode) *Tree {
	r.UpdateIds()
	url := "https://dedis.epfl.ch/tree/" + il.Id.String() + r.Id.String()
	return &Tree{
		IdList: il,
		Root:   r,
		Id:     uuid.NewV5(uuid.NamespaceURL, url),
	}
}

// NewTreeFromMarshal takes a slice of bytes and an IdentityList to re-create
// the original tree
func NewTreeFromMarshal(buf []byte, il *IdentityList) (*Tree, error) {
	tp, pm, err := network.UnmarshalRegisteredType(buf,
		network.DefaultConstructors(edwards.NewAES128SHA256Ed25519(false)))
	if err != nil {
		return nil, err
	}
	if tp != TreeMarshalType {
		return nil, errors.New("Didn't receive TreeMarshal-struct")
	}
	dbg.Lvl4("TreeMarshal is", pm.(TreeMarshal))
	return pm.(TreeMarshal).MakeTree(il)
}

// Marshal creates a simple binary-representation of the tree containing only
// the ids of the elements. Use NewTreeFromMarshal to get back the original
// tree
func (t *Tree) Marshal() ([]byte, error) {
	treeM := &TreeMarshal{
		Node:     t.Id,
		Identity: t.IdList.Id,
	}
	treeM.Children = append(treeM.Children, TreeMarshalCopyTree(t.Root))
	dbg.Lvlf4("TreeMarshal is %+v", treeM)
	buf, err := network.MarshalRegisteredType(treeM)
	return buf, err
}

// Equal verifies if the given tree is equal
func (t *Tree) Equal(t2 *Tree) bool {
	if t.Id != t2.Id || t.IdList.Id != t2.IdList.Id {
		dbg.Lvl4("Ids of trees don't match")
		return false
	}
	return t.Root.Equal(t2.Root)
}

// String writes the definition of the tree
func (t *Tree) String() string {
	return fmt.Sprintf("TreeId:%s - IdentityListId:%s - RootId:%s",
		t.Id, t.IdList.Id, t.Root.Id)
}

// TreeMarshalCopyTree takes a TreeNode and returns a corresponding
// TreeMarshal
func TreeMarshalCopyTree(tr *TreeNode) *TreeMarshal {
	tm := &TreeMarshal{
		Node:     tr.Id,
		Identity: tr.NodeId.Id,
	}
	for _, c := range tr.Children {
		tm.Children = append(tm.Children,
			TreeMarshalCopyTree(c))
	}
	return tm
}

// MakeTree creates a tree given an IdentityList
func (tm TreeMarshal) MakeTree(il *IdentityList) (*Tree, error) {
	if il.Id != tm.Identity {
		return nil, errors.New("Not correct IdentityList-Id")
	}
	tree := &Tree{
		Id:     tm.Node,
		IdList: il,
	}
	tree.Root = tm.Children[0].MakeTreeFromList(il)
	return tree, nil
}

// MakeTreeFromList creates a sub-tree given an IdentityList
func (tm *TreeMarshal) MakeTreeFromList(il *IdentityList) *TreeNode {
	tn := &TreeNode{
		Id:     tm.Node,
		NodeId: il.Search(tm.Identity),
	}
	for _, c := range tm.Children {
		tn.Children = append(tn.Children, c.MakeTreeFromList(il))
	}
	return tn
}

// TreeMarshal is used to send and receive a tree-structure without having
// to copy the whole nodelist
type TreeMarshal struct {
	// This is the UUID of the corresponding TreeNode, or the Tree-Id for the
	// top-node
	Node uuid.UUID
	// This is the UUID of the Identity, except for the top-node, where this
	// is the IdentityList-Id
	Identity uuid.UUID
	// All children from this tree. The top-node only has one child, which is
	// the root
	Children []*TreeMarshal
}

// A PeerList is a list of Identity we choose to run  some tree on it ( and
// therefor some protocols)
type IdentityList struct {
	Id   uuid.UUID
	List []*Identity
}

// NewIdentityList creates a new identity from a list of identities. It also
// adds a UUID which is randomly chosen.
func NewIdentityList(ids []*Identity) *IdentityList {
	url := "https://dedis.epfl.ch/identitylist/"
	for _, i := range ids {
		url += i.Id.String()
	}
	return &IdentityList{
		List: ids,
		Id:   uuid.NewV5(uuid.NamespaceURL, url),
	}
}

// Search looks for a corresponding UUID and returns that identity
func (il *IdentityList) Search(uuid uuid.UUID) *Identity {
	for _, i := range il.List {
		if i.Id == uuid {
			return i
		}
	}
	return nil
}

// TreeNode is one node in the tree
type TreeNode struct {
	// The Id represents that node of the tree
	Id uuid.UUID
	// The NodeID points to the corresponding host. One given host
	// can be used more than once in a tree.
	NodeId   *Identity
	Parent   *TreeNode
	Children []*TreeNode
}

// Check if it can communicate with parent or children
func (t *TreeNode) IsConnectedTo(id *Identity) bool {
	if t.Parent != nil && t.Parent.NodeId == id {
		return true
	}

	for i := range t.Children {
		if t.Children[i].NodeId == id {
			return true
		}
	}
	return false
}

// AddChild adds a child to this tree-node. Once the tree is set up, the
// function 'UpdateIds' should be called
func (t *TreeNode) AddChild(c *TreeNode) {
	t.Children = append(t.Children, c)
	c.Parent = t
}

// UpdateIds should be called on the root-node, so that it recursively
// calculates the whole tree as a merkle-tree
func (t *TreeNode) UpdateIds() {
	url := "https://dedis.epfl.ch/treenode/" + t.NodeId.Id.String()
	for _, child := range t.Children {
		child.UpdateIds()
		url += child.Id.String()
	}
	t.Id = uuid.NewV5(uuid.NamespaceURL, url)
}

// Equal tests if that node is equal to the given node
func (t *TreeNode) Equal(t2 *TreeNode) bool {
	if t.Id != t2.Id || t.NodeId.Id != t2.NodeId.Id {
		dbg.Lvl4("TreeNode: ids are not equal")
		return false
	}
	if len(t.Children) != len(t2.Children) {
		dbg.Lvl4("TreeNode: number of children are not equal")
		return false
	}
	for i, c := range t.Children {
		if !c.Equal(t2.Children[i]) {
			dbg.Lvl4("TreeNode: children are not equal")
			return false
		}
	}
	return true
}

// NewTreeNode creates a new TreeNode with the proper Id
func NewTreeNode(ni *Identity) *TreeNode {
	tn := &TreeNode{
		NodeId:   ni,
		Parent:   nil,
		Children: make([]*TreeNode, 0),
	}
	tn.UpdateIds()
	return tn
}

// String returns the current treenode's Id as a string.
func (t *TreeNode) String() string {
	return string(t.Id.String())
}

// Stringify returns a string containing the whole tree.
func (t *TreeNode) Stringify() string {
	var buf bytes.Buffer
	var lastDepth int
	fn := func(d int, n *TreeNode) {
		if d > lastDepth {
			buf.Write([]byte("\n\n"))
		} else {
			buf.Write([]byte(n.Id.String()))
		}
	}
	t.Visit(0, fn)
	return buf.String()
}

// Visit is a recursive function that allows for depth-first calling on all
// nodes
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
	Id   uuid.UUID
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
		Id:   id.Id,
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
		Id:   id.Id,
		List: ids,
	}
}

const (
	TopologyType = iota + 200
	TreeNodeType
	TreeMarshalType
	TreeType
	IdentityType
	IdentityListType
)

/*
Id is not used for the moment, rather a static, random UUID is used.
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

func (t *Tree) Id() UUID {
	h := NewHashFunc()
	h.Write([]byte(t.IdList.Id))
	h.Write([]byte(t.Root.Id()))
	return UUID(h.Sum(nil))
}

// generateId is not used for the moment, as we decided to use UUIDs, which
// are random. But perhaps it would be a good idea to switch back to
// something depending on public-key hashes anyway.
func generateId(ids []*Identity) UUID {
	h := NewHashFunc()
	for _, i := range ids {
		b, _ := i.Public.MarshalBinary()
		h.Write(b)
	}
	return UUID(h.Sum(nil))
}


*/

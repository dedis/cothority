// topology is a general
package sda

import (
	"errors"
	"fmt"
	"net"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

// In this file we define the main structures used for a running protocol
// instance. First there is the Entity struct: it represents the Entity of
// someone, a server over the internet, mainly tied by its public key.
// The tree contains the peerId which is the ID given to a an Entity / server
// during one protocol instance. A server can have many peerId in one tree.
// ProtocolInstance needs to know:
// - which EntityList we are using ( a selection of proper servers )
// - which Tree we are using.
// - The overlay network: a mapping from PeerId
// It contains the PeerId of the parent and the sub tree of the children.

func init() {
	network.RegisterMessageType(Tree{})
	network.RegisterMessageType(tbmStruct{})
}

// Tree is a topology to be used by any network layer/host layer
// It contains the peer list we use, and the tree we use
type Tree struct {
	Id         TreeID
	EntityList *EntityList
	Root       *TreeNode
}

// TreeID uniquely identifies a Tree struct in the sda framework.
type TreeID uuid.UUID

// Equals returns true if and only if the given TreeID equals the current one.
func (tId TreeID) Equals(tId2 TreeID) bool {
	return uuid.Equal(uuid.UUID(tId), uuid.UUID(tId2))
}

// String returns a canonical representation of the TreeID.
func (tId TreeID) String() string {
	return uuid.UUID(tId).String()
}

// NewTree creates a new tree using the entityList and the root-node. It
// also generates the id.
func NewTree(el *EntityList, r *TreeNode) *Tree {
	url := network.NamespaceURL + "tree/" + el.Id.String() + r.Id.String()
	t := &Tree{
		EntityList: el,
		Root:       r,
		Id:         TreeID(uuid.NewV5(uuid.NamespaceURL, url)),
	}
	// network.Suite used for the moment => explicit mark that something is
	// wrong and that needs to be changed !
	t.computeSubtreeAggregate(network.Suite, r)
	return t
}

// NewTreeFromMarshal takes a slice of bytes and an EntityList to re-create
// the original tree
func NewTreeFromMarshal(buf []byte, el *EntityList) (*Tree, error) {
	tp, pm, err := network.UnmarshalRegisteredType(buf,
		network.DefaultConstructors(network.Suite))
	if err != nil {
		return nil, err
	}
	if tp != TreeMarshalTypeID {
		return nil, errors.New("Didn't receive TreeMarshal-struct")
	}
	t, err := pm.(TreeMarshal).MakeTree(el)
	t.computeSubtreeAggregate(network.Suite, t.Root)
	return t, err
}

// MakeTreeMarshal creates a replacement-tree that is safe to send: no
// parent (creates loops), only sends ids (not send the entityList again)
func (t *Tree) MakeTreeMarshal() *TreeMarshal {
	if t.EntityList == nil {
		return &TreeMarshal{}
	}
	treeM := &TreeMarshal{
		TreeId:       t.Id,
		EntityListID: t.EntityList.Id,
	}
	treeM.Children = append(treeM.Children, TreeMarshalCopyTree(t.Root))
	return treeM
}

// Marshal creates a simple binary-representation of the tree containing only
// the ids of the elements. Use NewTreeFromMarshal to get back the original
// tree
func (t *Tree) Marshal() ([]byte, error) {
	buf, err := network.MarshalRegisteredType(t.MakeTreeMarshal())
	return buf, err
}

type tbmStruct struct {
	T  []byte
	EL *EntityList
}

// BinaryMarshaler does the same as Marshal
func (t *Tree) BinaryMarshaler() ([]byte, error) {
	bt, err := t.Marshal()
	if err != nil {
		return nil, err
	}
	tbm := &tbmStruct{
		T:  bt,
		EL: t.EntityList,
	}
	b, err := network.MarshalRegisteredType(tbm)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// BinaryUnmarshaler takes a TreeMarshal and stores it in the tree
func (t *Tree) BinaryUnmarshaler(b []byte) error {
	_, m, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	tbm, ok := m.(tbmStruct)
	if !ok {
		return errors.New("Didn't find TBMstruct")
	}
	tree, err := NewTreeFromMarshal(tbm.T, tbm.EL)
	if err != nil {
		return err
	}
	t.EntityList = tbm.EL
	t.Id = tree.Id
	t.Root = tree.Root
	return nil
}

// Equal verifies if the given tree is equal
func (t *Tree) Equal(t2 *Tree) bool {
	if t.Id != t2.Id || t.EntityList.Id != t2.EntityList.Id {
		dbg.Lvl4("Ids of trees don't match")
		return false
	}
	return t.Root.Equal(t2.Root)
}

// String writes the definition of the tree
func (t *Tree) String() string {
	return fmt.Sprintf("TreeId:%s - EntityListId:%s - RootId:%s",
		t.Id, t.EntityList.Id, t.Root.Id)
}

// Dump returns string about the tree
func (t *Tree) Dump() string {
	ret := "Tree " + t.Id.String() + " is:"
	t.Root.Visit(0, func(d int, tn *TreeNode) {
		if tn.Parent != nil {
			ret += fmt.Sprintf("\n%2d - %s/%s has parent %s/%s", d,
				tn.Id, tn.Entity.Addresses,
				tn.Parent.Id, tn.Parent.Entity.Addresses)
		} else {
			ret += fmt.Sprintf("\n%s/%s is root", tn.Id, tn.Entity.Addresses)
		}
	})
	return ret
}

// Search searches the Tree for the given TreeNodeID and returns the corresponding TreeNode
func (t *Tree) Search(tn TreeNodeID) (ret *TreeNode) {
	found := func(d int, tns *TreeNode) {
		if tns.Id == tn {
			ret = tns
		}
	}
	t.Root.Visit(0, found)
	return ret
}

// List returns a list of TreeNodes generated by DFS-iterating the Tree
func (t *Tree) List() (ret []*TreeNode) {
	ret = make([]*TreeNode, 0)
	add := func(d int, tns *TreeNode) {
		ret = append(ret, tns)
	}
	t.Root.Visit(0, add)
	return ret
}

// IsBinary returns true if every node has two or no children
func (t *Tree) IsBinary(root *TreeNode) bool {
	return t.IsNary(root, 2)
}

// IsNary returns true if every node has two or no children
func (t *Tree) IsNary(root *TreeNode, N int) bool {
	nChild := len(root.Children)
	if nChild != N && nChild != 0 {
		dbg.Lvl3("Only", nChild, "children for", root.Id)
		return false
	}
	for _, c := range root.Children {
		if !t.IsNary(c, N) {
			return false
		}
	}
	return true
}

// Size returns the number of all TreeNodes
func (t *Tree) Size() int {
	size := 0
	t.Root.Visit(0, func(d int, tn *TreeNode) {
		size += 1
	})
	return size
}

// UsesList returns true if all Entities of the list are used at least once
// in the tree
func (t *Tree) UsesList() bool {
	nodes := t.List()
	for _, p := range t.EntityList.List {
		found := false
		for _, n := range nodes {
			if n.Entity.ID == p.ID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// computeSubtreeAggregate will compute the aggregate subtree public key for
// each node of the tree.
// root is the root of the subtree we want to compute the aggregate for
// recursive function so it will go down to the leaves then go up to the root
// Return the aggregate sub tree public key for this root (and compute each sub
// aggregate public key for each of the children).
func (t *Tree) computeSubtreeAggregate(suite abstract.Suite, root *TreeNode) abstract.Point {
	aggregate := suite.Point().Add(suite.Point().Null(), root.Entity.Public)
	// DFS search
	for _, ch := range root.Children {
		aggregate = aggregate.Add(aggregate, t.computeSubtreeAggregate(suite, ch))
	}

	// sets the field
	root.PublicAggregateSubTree = aggregate
	return aggregate
}

// TreeMarshal is used to send and receive a tree-structure without having
// to copy the whole nodelist
type TreeMarshal struct {
	// This is the UUID of the corresponding TreeNode
	TreeNodeId TreeNodeID
	// TreeId identifies the Tree for the top-node
	TreeId TreeID
	// This is the UUID of the Entity, except
	EntityId network.EntityID
	// for the top-node this contains the EntityList's ID
	EntityListID EntityListID
	// All children from this tree. The top-node only has one child, which is
	// the root
	Children []*TreeMarshal
}

func (tm *TreeMarshal) String() string {
	s := fmt.Sprintf("%v", tm.EntityId)
	s += "\n"
	for i := range tm.Children {
		s += tm.Children[i].String()
	}
	return s
}

// ID of TreeMarshal message as registered in network
var TreeMarshalTypeID = network.RegisterMessageType(TreeMarshal{})

// TreeMarshalCopyTree takes a TreeNode and returns a corresponding
// TreeMarshal
func TreeMarshalCopyTree(tr *TreeNode) *TreeMarshal {
	tm := &TreeMarshal{
		TreeNodeId: tr.Id,
		EntityId:   tr.Entity.ID,
	}
	for i := range tr.Children {
		tm.Children = append(tm.Children,
			TreeMarshalCopyTree(tr.Children[i]))
	}
	return tm
}

// MakeTree creates a tree given an EntityList
func (tm TreeMarshal) MakeTree(el *EntityList) (*Tree, error) {
	if el.Id != tm.EntityListID {
		return nil, errors.New("Not correct EntityList-Id")
	}
	tree := &Tree{
		Id:         tm.TreeId,
		EntityList: el,
	}
	tree.Root = tm.Children[0].MakeTreeFromList(nil, el)
	tree.computeSubtreeAggregate(network.Suite, tree.Root)
	return tree, nil
}

// MakeTreeFromList creates a sub-tree given an EntityList
func (tm *TreeMarshal) MakeTreeFromList(parent *TreeNode, el *EntityList) *TreeNode {
	idx, ent := el.Search(tm.EntityId)
	tn := &TreeNode{
		Parent:    parent,
		Id:        tm.TreeNodeId,
		Entity:    ent,
		EntityIdx: idx,
	}
	for _, c := range tm.Children {
		tn.Children = append(tn.Children, c.MakeTreeFromList(tn, el))
	}
	return tn
}

// An EntityList is a list of Entity we choose to run  some tree on it ( and
// therefor some protocols)
type EntityList struct {
	Id EntityListID
	// TODO make that a map so search is O(1)
	// List is the List of actual "entities"
	// Be careful if you access it in go-routines (not safe by default)
	List []*network.Entity
	// Aggregate public key
	Aggregate abstract.Point
}

// EntityListID uniquely identifies an EntityList
type EntityListID uuid.UUID

// String returns the default representation of the ID (wrapper around
// uuid.UUID.String()
func (elId EntityListID) String() string {
	return uuid.UUID(elId).String()
}

// ID of EntityList message as registered in network
var EntityListTypeID = network.RegisterMessageType(EntityList{})

// NewEntityList creates a new Entity from a list of entities. It also
// adds a UUID which is randomly chosen.
func NewEntityList(ids []*network.Entity) *EntityList {
	// compute the aggregate key already
	agg := network.Suite.Point().Null()
	for _, e := range ids {
		agg = agg.Add(agg, e.Public)
	}
	return &EntityList{
		List:      ids,
		Aggregate: agg,
		Id:        EntityListID(uuid.NewV4()),
	}
}

// Search searches the EntityList for the given EntityID and returns the
// corresponding Entity.
func (el *EntityList) Search(eId network.EntityID) (int, *network.Entity) {
	for i, e := range el.List {
		if e.ID == eId {
			return i, e
		}
	}
	return 0, nil
}

// Get simply returns the entity that is stored at that index in the entitylist
// returns nil if index error
func (el *EntityList) Get(idx int) *network.Entity {
	if idx < 0 || idx > len(el.List) {
		return nil
	}
	return el.List[idx]
}

// GenerateBigNaryTree creates a tree where each node has N children.
// It will make a tree with exactly 'nodes' elements, regardless of the
// size of the EntityList. If 'nodes' is bigger than the number of elements
// in the EntityList, it will add some or all elements in the EntityList
// more than once.
// If the length of the EntityList is equal to 'nodes', it is guaranteed that
// all Entities from the EntityList will be used in the tree.
// However, for some configurations it is impossible to use all Entities from
// the EntityList and still avoid having a parent and a child from the same
// host. In this case use-all has preference over not-the-same-host.
func (el *EntityList) GenerateBigNaryTree(N, nodes int) *Tree {
	// list of which hosts are already used
	used := make([]bool, len(el.List))
	ilLen := len(el.List)
	// only use all Entities if we have the same number of nodes and hosts
	useAll := ilLen == nodes
	root := NewTreeNode(0, el.List[0])
	used[0] = true
	levelNodes := []*TreeNode{root}
	totalNodes := 1
	elIndex := 1 % ilLen
	for totalNodes < nodes {
		newLevelNodes := make([]*TreeNode, len(levelNodes)*N)
		newLevelNodesCounter := 0
		for i, parent := range levelNodes {
			children := (nodes - totalNodes) * (i + 1) / len(levelNodes)
			if children > N {
				children = N
			}
			parent.Children = make([]*TreeNode, children)
			parentHost, _, _ := net.SplitHostPort(parent.Entity.Addresses[0])
			for n := 0; n < children; n++ {
				// Check on host-address, so that no child is
				// on the same host as the parent.
				childHost, _, _ := net.SplitHostPort(el.List[elIndex].Addresses[0])
				elIndexFirst := elIndex
				notSameHost := true
				for (notSameHost && childHost == parentHost && ilLen > 1) ||
					(useAll && used[elIndex]) {
					elIndex = (elIndex + 1) % ilLen
					if useAll && used[elIndex] {
						// In case we searched all Entities,
						// give up on finding another host, but
						// keep using all Entities
						if elIndex == elIndexFirst {
							notSameHost = false
						}
						continue
					}
					// If we tried all hosts, it means we're using
					// just one hostname, as we didn't find any
					// other name
					if elIndex == elIndexFirst {
						break
					}
					childHost, _, _ = net.SplitHostPort(el.List[elIndex].Addresses[0])
				}
				child := NewTreeNode(elIndex, el.List[elIndex])
				used[elIndex] = true
				elIndex = (elIndex + 1) % ilLen
				totalNodes += 1
				parent.Children[n] = child
				child.Parent = parent
				newLevelNodes[newLevelNodesCounter] = child
				newLevelNodesCounter += 1
			}
		}
		levelNodes = newLevelNodes[:newLevelNodesCounter]
	}
	return NewTree(el, root)
}

// GenerateNaryTree creates a tree where each node has N children.
// The first element of the EntityList will be the root element.
func (el *EntityList) GenerateNaryTree(N int) *Tree {
	root := el.addNary(nil, N, 0, len(el.List)-1)
	return NewTree(el, root)
}

// addNary is a recursive function to create the binary tree
func (el *EntityList) addNary(parent *TreeNode, N, start, end int) *TreeNode {
	if start <= end && end < len(el.List) {
		node := NewTreeNode(start, el.List[start])
		if parent != nil {
			node.Parent = parent
			parent.Children = append(parent.Children, node)
		}
		diff := end - start
		for n := 0; n < N; n++ {
			s := diff * n / N
			e := diff * (n + 1) / N
			el.addNary(node, N, start+s+1, start+e)
		}
		return node
	} else {
		return nil
	}
}

// GenerateBinaryTree creates a binary tree out of the EntityList
// out of it. The first element of the EntityList will be the root element.
func (el *EntityList) GenerateBinaryTree() *Tree {
	return el.GenerateNaryTree(2)
}

// TreeNode is one node in the tree
type TreeNode struct {
	// The Id represents that node of the tree
	Id TreeNodeID
	// The Entity points to the corresponding host. One given host
	// can be used more than once in a tree.
	Entity *network.Entity
	// EntityIdx is the index in the EntityList where the `Entity` is located
	EntityIdx int
	// Parent link
	Parent *TreeNode
	// Children links
	Children []*TreeNode
	// Aggregate public key for *this* subtree,i.e. this node's public key + the
	// aggregate of all its children's aggregate public key
	PublicAggregateSubTree abstract.Point
}

// TreeNodeID identifies a given TreeNode struct in the sda framework.
type TreeNodeID uuid.UUID

// String returns a canonical representation of the TreeNodeID.
func (tId TreeNodeID) String() string {
	return uuid.UUID(tId).String()
}

// Equals returns true if and only if the given TreeNodeID equals the current
// one.
func (tId TreeNodeID) Equals(tId2 TreeNodeID) bool {
	return uuid.Equal(uuid.UUID(tId), uuid.UUID(tId2))
}

// Name returns a human readable representation of the TreeNode (IP address).
func (t *TreeNode) Name() string {
	return t.Entity.First()
}

var _ = network.RegisterMessageType(TreeNode{})

// NewTreeNode creates a new TreeNode with the proper Id
func NewTreeNode(entityIdx int, ni *network.Entity) *TreeNode {
	tn := &TreeNode{
		Entity:    ni,
		EntityIdx: entityIdx,
		Parent:    nil,
		Children:  make([]*TreeNode, 0),
		Id:        TreeNodeID(uuid.NewV4()),
	}
	return tn
}

// IsConnectedTo checks if the TreeNode can communicate with its parent or
// children.
func (t *TreeNode) IsConnectedTo(e *network.Entity) bool {
	if t.Parent != nil && t.Parent.Entity.Equal(e) {
		return true
	}

	for i := range t.Children {
		if t.Children[i].Entity.Equal(e) {
			return true
		}
	}
	return false
}

// IsLeaf returns true for a node without children
func (t *TreeNode) IsLeaf() bool {
	return len(t.Children) == 0
}

// IsRoot returns true for a node without a parent
func (t *TreeNode) IsRoot() bool {
	return t.Parent == nil
}

// IsInTree - verifies if the TreeNode is in the given Tree
func (t *TreeNode) IsInTree(tree *Tree) bool {
	root := *t
	for root.Parent != nil {
		root = *root.Parent
	}
	return tree.Root.Id == root.Id
}

// AddChild adds a child to this tree-node.
func (t *TreeNode) AddChild(c *TreeNode) {
	t.Children = append(t.Children, c)
	c.Parent = t
}

// Equal tests if that node is equal to the given node
func (t *TreeNode) Equal(t2 *TreeNode) bool {
	if t.Id != t2.Id || t.Entity.ID != t2.Entity.ID {
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

// String returns the current treenode's Id as a string.
func (t *TreeNode) String() string {
	return string(t.Id.String())
}

// Visit is a recursive function that allows for depth-first calling on all
// nodes
func (t *TreeNode) Visit(firstDepth int, fn func(depth int, n *TreeNode)) {
	fn(firstDepth, t)
	for _, c := range t.Children {
		c.Visit(firstDepth+1, fn)
	}
}

// EntityListToml is the struct can can embedded EntityToml to be written in a
// toml file
type EntityListToml struct {
	Id   EntityListID
	List []*network.EntityToml
}

// Toml returns the toml-writable version of this entityList
func (el *EntityList) Toml(suite abstract.Suite) *EntityListToml {
	ids := make([]*network.EntityToml, len(el.List))
	for i := range el.List {
		ids[i] = el.List[i].Toml(suite)
	}
	return &EntityListToml{
		Id:   el.Id,
		List: ids,
	}
}

// EntityList returns the Id list from this toml read struct
func (elt *EntityListToml) EntityList(suite abstract.Suite) *EntityList {
	ids := make([]*network.Entity, len(elt.List))
	for i := range elt.List {
		ids[i] = elt.List[i].Entity(suite)
	}
	return &EntityList{
		Id:   elt.Id,
		List: ids,
	}
}

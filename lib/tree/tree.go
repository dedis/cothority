package tree

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
	"hash"
)

/*
The tree library holds all hosts and allows for different ways to
retrieve the hosts - either in a n-ary tree, broadcast or just
 a selection.
*/

type TreeId struct {
	TreeHash hashid.HashId
}

// TreeEntry is one entry in the tree and linked to it's parent and
// the tree-structure it's in.
type TreeNode struct {
	// TreeId represents the ID of the tree that is computed from the list of
	// treenodes.
	TreeId *TreeId
	// A list of all child-nodes - the indexes are relative to the PeerList
	Children []*TreeNode
	// The parent node - or nil if this is the root
	Parent *TreeNode
	// The actual Peer stored in this node.
	Peer *Peer
}

// Init a tree node from the ID and a peer
func (tn *TreeNode) init(p *Peer) *TreeNode {
	tn = &TreeNode{
		Children: make([]*TreeNode, 0),
		Peer:     p,
	}
	return tn
}

// Returns a fresh new TreeNode
func NewTreeNode(p *Peer) *TreeNode {
	return new(TreeNode).init(p)
}

// AddChild appends a node into the child list of this treenode. It also updates
// the Parent pointer of the child.
func (te *TreeNode) AddChild(tn *TreeNode) {
	te.Children = append(te.Children, tn)
	tn.Parent = te
}

// VisistsBFS will visits the tree BFS style calling the given function for each
// node encountered from the root.
func VisitsBFS(root *TreeNode, fn func(*TreeNode)) {
	fn(root)
	for _, child := range root.Children {
		VisitsBFS(child, fn)
	}
}

// CountRec counts the number of children recursively
func (te *TreeNode) Count() int {
	nbr := 0
	VisitsBFS(te, func(tn *TreeNode) {
		nbr += 1
	})
	return nbr
}

// Write simply write the peer representation into the writer. Used for hashing.
func (tn *TreeNode) Bytes() []byte {
	buf := tn.Peer.Bytes()
	// if we have a parent
	if tn.Parent != nil {
		// we include the link from the parent to us in the hash
		buf = append(buf, tn.Parent.Peer.Bytes()...)
	}
	return buf
}

// Id will hash its whole topology to produce an TreeId. It will set the treeId
// field for each nodes in its topology
func (tn *TreeNode) GenId(hashFunc hash.Hash) hashid.HashId {
	tid := &TreeId{}
	// Visits the whole tree
	VisitsBFS(tn, func(node *TreeNode) {
		// The node write itselfs
		hashFunc.Write(node.Bytes())
		// then sets the right fields
		node.TreeId = tid
	})
	// Set the hashid
	tid.TreeHash = hashid.HashId(hashFunc.Sum(nil))
	return tid.TreeHash
}

func (tn *TreeNode) Id() hashid.HashId {
	return tn.TreeId.TreeHash
}

// NewNaryTree creates a regular tree with a branching factor bf from the list
// of peers "peers". It returns the root.
func NewNaryTree(s abstract.Suite, bf int, peers []*Peer) *TreeNode {
	if len(peers) < 1 {
		return nil
	}
	dbg.Lvl3("NewNaryTree Called with", len(peers), "peers and bf =", bf)
	root := NewTreeNode(peers[0])
	var index int = 1
	bfs := make([]*TreeNode, 1)
	bfs[0] = root
	for len(bfs) > 0 && index < len(peers) {
		t := bfs[0]
		t.Children = make([]*TreeNode, 0)
		lbf := 0
		// create space for enough children
		// init them
		for lbf < bf && index < len(peers) {
			child := NewTreeNode(peers[index])
			// append the children to the list of trees to visit
			bfs = append(bfs, child)
			t.Children = append(t.Children, child)
			index += 1
			lbf += 1
		}
		bfs = bfs[1:]
	}
	// Compute the tree id
	root.GenId(s.Hash())
	return root
}

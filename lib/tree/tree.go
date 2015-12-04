package tree

import (
	"github.com/dedis/cothority/lib/dbg"
)

/*
The tree library holds all hosts and allows for different ways to
retrieve the hosts - either in a n-ary tree, broadcast or just
 a selection.
*/

// Tree holds the structure about the current tree
type Tree struct {
	// The top-most peer of this tree
	Root *TreeEntry
	// The hash-id of this tree - same for the whole tree
	Hash []byte
	// The hash of the peer-list
	HashPL []byte
	// The branching factor
	BF int
}

// TreeEntry is one entry in the tree and linked to it's parent and
// the tree-structure it's in.
type TreeEntry struct {
	// Pointer to the tree-structure we're in
	Tree *Tree
	// A list of all child-nodes - the indexes are relative to the PeerList
	Children []*TreeEntry
	// The parent node - or nil if this is the root
	Parent *TreeEntry
	// The actual Peer stored in this node.
	Peer *Peer
}

// AddRoot creates the root-entry for the tree
func (t *Tree) AddRoot(peer *Peer) *TreeEntry {
	t.Root = &TreeEntry{
		Tree:     t,
		Children: make([]*TreeEntry, 0),
		Peer:     peer,
	}
	return t.Root
}

// AddChild adds a Children to the TreeEntry and returns the
// children added
func (te *TreeEntry) AddChild(peer *Peer) *TreeEntry {
	child := &TreeEntry{
		Tree:     te.Tree,
		Parent:   te,
		Peer:     peer,
		Children: make([]*TreeEntry, 0),
	}
	te.Children = append(te.Children, child)
	return child
}

// Count returns the number of children of that Tree
func (t *Tree) Count() int {
	return t.Root.CountRec()
}

// CountRec counts the number of children recursively
func (te *TreeEntry) CountRec() int {
	dbg.Lvlf3("Children are: %+v", te.Children)
	nbr := 1
	for _, t := range te.Children {
		nbr += t.CountRec()
	}
	return nbr
}

// NewNaryTree creates a tree with branching factor bf and attaches it
// to the TreeEntry
func (te *TreeEntry) NewNaryTree(peers []*Peer) {
	bf := te.Tree.BF
	numberLeft := len(peers)
	dbg.Lvl3("Called with", numberLeft, "peers and bf =", bf)

	start := 0
	for b := 1; b <= bf; b++ {
		// Remember: slice-ranges are exclusive of the end. So
		// len(peers[0..1]) == 1!
		end := b * numberLeft / bf
		if end > start {
			dbg.Lvl3(b, ": Creating children", start, ":", end, "of", numberLeft)
			nte := te.AddChild(peers[start])
			if end > (start + 1) {
				nte.NewNaryTree(peers[start+1 : end])
			}
			start = end
		}
	}
	dbg.Lvlf3("Finished node %+v", te)
}

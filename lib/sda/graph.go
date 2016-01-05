/*
Graph holds the structure for both graphs and peer-lists.
*/

package sda

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
)

/*
A graph represents a collection of peers that are connected in a
uni-directional way.
*/
type Graph struct {
	id       hashid.HashId
	root     *TreePeer
	peerList *PeerList
}

/*
TreePeer represents a double-linked list to parent and children. The 'peer'-
field can point to the same peer for different 'TreePeer's.
*/
type TreePeer struct {
	// The Graph this TreePeer belongs to
	graph *Graph
	// The id of that TreePeer
	id       hashid.HashId
	parent   *TreePeer
	children []*TreePeer
	peer     string
}

/*
Functions to return the fields of the array in a readonly-fashion
*/
func (g *TreePeer) Graph() *Graph {
	return g.graph
}
func (g *TreePeer) ID() hashid.HashId {
	return g.id
}
func (g *TreePeer) Parent() *TreePeer {
	return g.parent
}
func (g *TreePeer) Children() []*TreePeer {
	return g.children
}

/*
IsLeaf is true for the leaf of the tree
*/
func (g *TreePeer) IsLeaf() bool {
	return len(g.children) == 0
}

/*
IsRoot is true for the leaf of the tree
*/
func (g *TreePeer) IsRoot() bool {
	return g.parent == nil
}

/*
IsIntermediate is true if we're neither Root nor Leaf
*/
func (g *TreePeer) IsIntermediate() bool {
	return !g.IsRoot() && g.IsLeaf()
}

/*
PeerList holds all peers under a common identity
*/
type PeerList struct {
	id    hashid.HashId
	peers map[string]*Peer
}

/*
Peer represents one Conode and holds it's public-key. It's ID is the
address of the peer, as this is unique.
*/
type Peer struct {
	// The "ip:port" of that peer
	address string
	// The public-key of that peer
	public abstract.Point
}

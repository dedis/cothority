/*
Graph holds the structure for both graphs and peer-lists.
*/

package sda

/*
TreePeer represents a double-linked list to parent and children. As a peer
can be represented more than once in the graph, it needs to be given as
an extra field.
*/
type TreePeer struct {
	// The id of the Graph
	graphID int
	// The id of that TreePeer
	id       int
	parent   *TreePeer
	children []*TreePeer
	peer     *Peer
}

/*
Functions to return the fields of the array in a readonly-fashion
*/
func (g *TreePeer) GraphID() int {
	return g.graphID
}
func (g *TreePeer) ID() int {
	return g.id
}
func (g *TreePeer) Parent() *Peer {
	return g.parent
}
func (g *TreePeer) Children() []*Peer {
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

type PeerList struct {
}

type Peer struct {
}

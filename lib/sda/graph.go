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
	ID       int
	Parent   *TreePeer
	Children []*TreePeer
	Peer     *Peer
}

func (g *TreePeer) Parent(p *Peer) *Peer {
	return g.Parent
}

func (g *TreePeer) Children(p *Peer) []*Peer {
	return g.Children
}

/*
IsLeaf is true for the leaf of the tree
*/
func (g *TreePeer) IsLeaf() bool {
	return len(g.Children) == 0
}

/*
IsRoot is true for the leaf of the tree
*/
func (g *TreePeer) IsRoot() bool {
	return g.Parent == nil
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

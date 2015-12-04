package tree

import "github.com/dedis/cothority/lib/dbg"

// Creates a tree of peers recursively
func NewNaryTree(peers []*Peer, bf int) *Node {
	numberLeft := len(peers) - 1
	dbg.Lvl3("Calling with", numberLeft, "peers and bf =", bf)

	node := &Node{}
	node.Peer = peers[0]
	node.Children = make([]*Node, 0)
	//node.Hash = node.Peer.PubKey
	start := 1
	for b := 1; b <= bf; b++ {
		end := b * numberLeft / bf
		if end > start {
			dbg.Lvl3("Creating children", start, "..", end, "of", numberLeft)
			node.Children = append(node.Children, NewNaryTree(peers[start:end+1], bf))
			start = end + 1
		}
	}
	dbg.Lvlf3("Returning node %+v", node)
	return node
}

func (node *Node) Count() int {
	dbg.Lvlf3("Children are: %+v", node.Children)
	nbr := 1
	for _, n := range node.Children {
		nbr += n.Count()
	}
	return nbr
}

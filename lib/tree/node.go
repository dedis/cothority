package tree

// Creates a tree of peers recursively
func NewNaryTree(peers []*Peer, bf int) *Node {
	number := len(peers) - 1
	if number == 0 {
		return nil
	}

	node := &Node{}
	node.Peer = peers[0]
	node.Children = make([]*Node, 0)
	//node.Hash = node.Peer.PubKey
	start := 1
	for b := 1; b <= bf; b++ {
		end := start + b*number/bf
		if end > start {
			node.Children = append(node.Children, NewNaryTree(peers[start:end], bf))
		}
		start = end
	}
	return node
}

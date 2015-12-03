package tree

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
)

/*
The tree library holds all hosts and allows for different ways to
retrieve the hosts - either in a n-ary tree, broadcast or just
 a selection.
*/

// Peer represents a Cothority-member
type Peer struct {
	// The hostname of the peer
	Name string
	// The port-number
	Port int
	// The public-key - if known
	PubKey abstract.Point
	// A network-connection - if already set up
	Conn network.Conn
}

// PeerList regroup a number of peers in a list. One peer can be
// member of more than one PeerList.
type PeerList struct {
	// A list of all peers that are part of this list
	Peers []*Peer
	// The hash-id of this list
	Hash hashid.HashId
}

// Node is double-linked to parent and children
type Node struct {
	// A list of all child-nodes
	Children []*Node
	// The parent node - or nil if this is the root
	Parent *Node
	// The actual Host stored in this node
	Peer *Peer
	// The hash-id of this tree - every sub-tree has its own hash-id
	Hash hashid.HashId
}

/*
Graph holds the structure for both graphs and peer-lists.
*/

package sda

import (
	"crypto/sha256"

	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
)

/*
A graph represents a collection of peers that are connected in a
uni-directional way.
*/
type Graph struct {
	id         hashid.HashId
	root       *TreePeer
	PeerListId hashid.HashId
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
	// peerId sth. like ip:port
	peerId string
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
	// id identifies the PeerList (id is used in Graph)
	Id hashid.HashId
	// the actual peer-list (address -> Peer):
	Peers map[string]*Peer
}

// NewPeerList creates a PeerList using the list of peers provided
func NewPeerList(peers map[string]*Peer) *PeerList {
	pl := &PeerList{
		Peers: peers,
	}
	pl.generateId()
	return pl
}

// generateId computes, sets, and returns the ID of this peer list
// basically, it is the sha256 hash of the concatenation of the binary
// representation of all its peers
func (pl *PeerList) generateId() hashid.HashId {
	hash := sha256.New()
	for _, p := range pl.Peers {
		pBytes, _ := p.Public.MarshalBinary()
		hash.Write(pBytes)
	}
	pl.Id = hashid.HashId(hash.Sum(nil))
	return pl.Id
}

// GenPeerList creates a PeerList using the given addresses.
// It will generate a new key pair and a unique id for each peer.
func GenPeerList(s abstract.Suite, adresses []string) *PeerList {
	peers := make(map[string]*Peer, len(adresses))
	for i := 0; i < len(adresses); i++ {
		keyPair := new(config.KeyPair)
		// generate random keys:
		keyPair.Gen(s, random.Stream)
		peers[adresses[i]] = NewPeer(adresses[i], keyPair.Public)
	}
	return NewPeerList(peers)
}

/*
Peer represents one Conode and holds it's public-key. It's ID is the
address of the peer, as this is unique.
*/
type Peer struct {
	// The "ip:port" of that peer
	Address string
	// The public-key of that peer
	Public abstract.Point
}

// NewPeer returns a fresh initialized peer struct
func NewPeer(address string, public abstract.Point) *Peer {
	return &Peer{
		Address: address,
		Public:  public,
	}
}

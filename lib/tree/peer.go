package tree

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"strconv"
)

// Peer represents a Cothority-member
type Peer struct {
	// The hostname of the peer
	Name string
	// The port-number
	Port int
	// The public-key
	PubKey abstract.Point
	// The private-key - mostly for ourselves
	PrivKey abstract.Secret
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
	// The suite used in this list
	Suite abstract.Suite
}

// NewPeerListLocalhost creates a list of n localhosts, starting
// with port p and random keys.
func NewPeerListLocalhost(s abstract.Suite, n, port int) *PeerList {
	pl := &PeerList{
		Peers: make([]*Peer, n),
		Suite: s,
	}

	// Pick a random generator with a seed of n and port
	rand := s.Cipher([]byte(strconv.Itoa(n * port)))
	for i := 0; i < n; i++ {
		privKey := s.Secret().Pick(rand)
		pl.Peers[i] = &Peer{
			Name:    "localhost",
			Port:    port + i,
			PrivKey: privKey,
			PubKey:  s.Point().Mul(nil, privKey),
		}
	}
	//pl.Hash = s.Secret().Pick(rand)
	return pl
}

// NewNaryTree creates a tree of peers recursively with branching
// factor bf. If bf = 2, it will create a binary tree.
func (pl *PeerList) NewNaryTree(bf int) *Tree {
	// Create a hash of (peerList.Hash || #peers || bf )
	hash := pl.Suite.Hash()
	hash.Write(pl.Hash)
	hash.Write([]byte(strconv.Itoa(len(pl.Peers))))
	hash.Write([]byte(strconv.Itoa(bf)))
	t := &Tree{
		Hash:   hash.Sum(nil),
		HashPL: pl.Hash,
		BF:     bf,
	}
	root := t.AddRoot(pl.Peers[0])
	root.NewNaryTree(pl.Peers[1:])
	return t
}

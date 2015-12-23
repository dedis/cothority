package tree

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
	"hash"
)

// Peer represents a Cothority-member
type Peer struct {
	// The hostname of the peer
	// ip-address:port
	Name string
	// The public-key
	Public abstract.Point
	// private key
	Secret abstract.Secret
}

// init initializes the peer structure
func (p *Peer) init(name string, public abstract.Point, secret abstract.Secret) *Peer {
	p = &Peer{
		Name:   name,
		Public: public,
		Secret: secret}
	return p
}

// NewPeer returns a fresh initialized peer struct
func NewPeer(name string, public abstract.Point, secret abstract.Secret) *Peer {
	return new(Peer).init(name, public, secret)
}

// write writes an byte representation of a peer used for hashing
func (p *Peer) Bytes() []byte {
	pbuf, _ := p.Public.MarshalBinary()
	return append(pbuf, []byte(p.Name)...)
}

// PeerList regroup a number of peers in a list. One peer can be
// member of more than one PeerList.
type PeerList struct {
	// A list of all peers that are part of this list
	Peers []*Peer
	// The hash-id of this list
	ListId hashid.HashId
	// The suite used in this list
	Suite abstract.Suite
}

// Computes, set, and returns the ID of this peer list
// basically, it is the hash of the concatenation of all its peers
func (pl *PeerList) GenId(hashFunc hash.Hash) hashid.HashId {
	for _, p := range pl.Peers {
		hashFunc.Write(p.Bytes())
	}
	pl.ListId = hashid.HashId(hashFunc.Sum(nil))
	return pl.ListId
}

func (pl *PeerList) Id() hashid.HashId {
	return pl.ListId
}

// init a peerlist
func (pl *PeerList) init(s abstract.Suite, peers []*Peer) *PeerList {
	pl = &PeerList{
		Peers: peers,
		Suite: s,
	}
	pl.GenId(s.Hash())
	return pl
}

func (pl *PeerList) Copy() PeerList {
	pl2 := PeerList{
		Suite:  pl.Suite,
		ListId: pl.ListId,
	}
	copy(pl2.Peers, pl.Peers)
	return pl2
}

// NewPeerList init a PeerList with this list of peers
func NewPeerList(s abstract.Suite, peers []*Peer) *PeerList {
	return new(PeerList).init(s, peers)
}

// GenPeerList creates a PeerList out of the names given. It will generate a new
// key pair for each peers.
func GenPeerList(s abstract.Suite, names []string) *PeerList {
	peers := make([]*Peer, len(names))
	for i := 0; i < len(names); i++ {
		keyPair := new(config.KeyPair)
		// gen keys Randomly
		keyPair.Gen(s, random.Stream)
		peers[i] = NewPeer(names[i], keyPair.Public, keyPair.Secret)
	}
	return NewPeerList(s, peers)
}

// NewNaryTree creates a tree of peers recursively with branching
// factor bf. If bf = 2, it will create a binary tree.
// It returns the root.
func (pl *PeerList) NewNaryTree(bf int) *Node {
	root := NewNaryTree(pl.Suite, bf, pl.Peers)
	return root
}

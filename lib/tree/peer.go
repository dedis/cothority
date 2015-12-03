package tree

import (
	"github.com/dedis/crypto/abstract"
	"strconv"
)

func NewPeerListLocalhost(s abstract.Suite, n, port int) *PeerList {
	pl := &PeerList{
		Peers: make([]*Peer, n),
	}

	// Pick a random generator with a seed of n and port
	rand := s.Cipher([]byte(strconv.Itoa(n * port)))
	for i := 0; i < n; i++ {
		privKey := s.Secret().Pick(rand)
		pl.Peers[i] = &Peer{
			Name:   "localhost",
			Port:   port + i,
			PubKey: s.Point().Mul(nil, privKey),
		}
	}
	//pl.Hash = s.Secret().Pick(rand)
	return pl
}

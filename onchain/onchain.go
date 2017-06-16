package onchain

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

type Peer struct {
	Secret abstract.Scalar
}

func NewPeer() *Peer {
	return &Peer{
		Secret: network.Suite.Scalar().Pick(random.Stream),
	}
}

type Acknowledge struct{}

// Everyone should have a share of the key and the public part should
// be available.
func (p *Peer) GenerateKey() {
}

// Multiplies the given point with the private secret
func (p *Peer) MulWithSecret(point abstract.Point) abstract.Point {
	return network.Suite.Point().Mul(point, p.Secret)
}

func LagrangeInterpolate(shares []abstract.Point) abstract.Point {
	if len(shares) > 1 {
		log.Fatal("Not yet implemented")
	}
	return shares[0]
}

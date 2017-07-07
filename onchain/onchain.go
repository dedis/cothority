package onchain

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/crypto.v0/share/dkg"
	"gopkg.in/dedis/onet.v1/log"
)

type Peer struct {
	Suite  abstract.Suite
	Secret abstract.Scalar
	DKG    *dkg.DistKeyGenerator
}

func NewPeer(s abstract.Suite) *Peer {
	return &Peer{
		Suite:  s,
		Secret: s.Scalar().Pick(random.Stream),
	}
}

type Acknowledge struct{}

// Public returns public key
func (p *Peer) Public() abstract.Point {
	return p.Suite.Point().Mul(nil, p.Secret)
}

// GenerateKey.
func (p *Peer) GenerateKey(publics []abstract.Point, t int) (err error) {
	p.DKG, err = dkg.NewDistKeyGenerator(p.Suite, p.Secret, publics,
		random.Stream, t)
	return
}

// Multiplies the given point with the private secret
func (p *Peer) MulWithSecret(point abstract.Point) abstract.Point {
	dks, err := p.DKG.DistKeyShare()
	log.ErrFatal(err)
	return p.Suite.Point().Mul(point, dks.Share.V)
}

func LagrangeInterpolate(shares []abstract.Point) abstract.Point {
	if len(shares) > 1 {
		log.Fatal("Not yet implemented")
	}
	return shares[0]
}

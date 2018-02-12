package lib

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/group/edwards25519"
	"github.com/dedis/kyber/xof/blake"
)

var (
	Suite  = edwards25519.NewBlakeSHA256Ed25519WithRand(blake.New(nil))
	Stream = Suite.RandomStream()
	Base   = Suite.Point().Base()
)

// RandomKeyPair creates a random public/private Diffie-Hellman key pair.
func RandomKeyPair() (x kyber.Scalar, X kyber.Point) {
	x = Suite.Scalar().Pick(Stream)
	X = Suite.Point().Mul(x, nil)
	return
}

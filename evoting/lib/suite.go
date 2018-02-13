package lib

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/group/edwards25519"
	"github.com/dedis/kyber/xof/blake"
)

var (
	// Suite is the Ed25519 curve.
	Suite = edwards25519.NewBlakeSHA256Ed25519WithRand(blake.New(nil))
	// Stream is used to generate random Ed25519 curve points.
	Stream = Suite.RandomStream()
)

// RandomKeyPair creates a random public/private Diffie-Hellman key pair.
func RandomKeyPair() (x kyber.Scalar, X kyber.Point) {
	x = Suite.Scalar().Pick(Stream)
	X = Suite.Point().Mul(x, nil)
	return
}

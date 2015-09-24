package sign

import (
	"crypto/cipher"
	"errors"

	"github.com/dedis/crypto/abstract"
)

// A basic, verifiable signature
type BasicSig struct {
	C abstract.Secret // challenge
	R abstract.Secret // response
}

// This simplified implementation of ElGamal Signatures is based on
// crypto/anon/sig.go
func ElGamalSign(suite abstract.Suite, random cipher.Stream, message []byte,
	privateKey abstract.Secret) BasicSig {

	// Create random secret v and public point commitment T
	v := suite.Secret().Pick(random)
	T := suite.Point().Mul(nil, v)

	// Create challenge c based on message and T
	c := hashElGamal(suite, message, T)

	// Compute response r = v - x*c
	r := suite.Secret()
	r.Mul(privateKey, c).Sub(v, r)

	// Return verifiable signature {c, r}
	sig := BasicSig{c, r}
	return sig
}

func ElGamalVerify(suite abstract.Suite, message []byte, publicKey abstract.Point,
	sig BasicSig) error {
	r := sig.R
	c := sig.C

	// Compute base**(r + x*c) == T
	var P, T abstract.Point
	P = suite.Point()
	T = suite.Point()
	T.Add(T.Mul(nil, r), P.Mul(publicKey, c))

	// Verify that the hash based on the message and T
	// matches the challange c from the signature
	c = hashElGamal(suite, message, T)
	if !c.Equal(sig.C) {
		return errors.New("invalid signature")
	}

	return nil
}

package main

import (
	"errors"
	. "github.com/dedis/cothority/lib/network_draft/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/cipher"
)

// Returns a secret that depends on on a message and a point
func hashSchnorr(suite abstract.Suite, message []byte, p abstract.Point) abstract.Secret {
	pb, _ := p.MarshalBinary()
	c := suite.Cipher(pb)
	c.Message(nil, nil, message)
	return suite.Secret().Pick(c)
}

// This simplified implementation of Schnorr Signatures is based on
// crypto/anon/sig.go
// The ring structure is removed and
// The anonimity set is reduced to one public key = no anonimity
func SchnorrSign(suite abstract.Suite, random cipher.Stream, message []byte,
	privateKey abstract.Secret) BasicSignature {

	// Create random secret v and public point commitment T
	v := suite.Secret().Pick(random)
	T := suite.Point().Mul(nil, v)

	// Create challenge c based on message and T
	c := hashSchnorr(suite, message, T)

	// Compute response r = v - x*c
	r := suite.Secret()
	r.Mul(privateKey, c).Sub(v, r)

	// Return verifiable si,gnature {c, r}
	// Verifier will be able to compute v = r + x*c
	// And check that hashElgamal for T and the message == c
	sig := BasicSignature{Chall: c, Resp: r}
	return sig
}

func SchnorrVerify(suite abstract.Suite, message []byte,
	signature BasicSignature) error {
	publicKey := signature.Pub
	r := signature.Resp
	c := signature.Chall

	// Compute base**(r + x*c) == T
	var P, T abstract.Point
	P = suite.Point()
	T = suite.Point()
	T.Add(T.Mul(nil, r), P.Mul(publicKey, c))

	// Verify that the hash based on the message and T
	// matches the challange c from the signature
	c = hashSchnorr(suite, message, T)
	if !c.Equal(signature.Chall) {
		return errors.New("invalid signature")
	}

	return nil
}

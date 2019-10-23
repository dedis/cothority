package lib

import (
	"errors"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/proof"
	"go.dedis.ch/kyber/v4/shuffle"
	"go.dedis.ch/kyber/v4/util/random"

	"go.dedis.ch/cothority/v4"
)

// Encrypt performs the ElGamal encryption algorithm.
func Encrypt(public kyber.Point, message []byte) (K, C kyber.Point) {
	M := cothority.Suite.Point().Embed(message, random.New())

	// ElGamal-encrypt the point to produce ciphertext (K,C).
	k := cothority.Suite.Scalar().Pick(random.New()) // ephemeral private key
	K = cothority.Suite.Point().Mul(k, nil)          // ephemeral DH public key
	S := cothority.Suite.Point().Mul(k, public)      // ephemeral DH shared secret
	C = S.Add(S, M)                                  // message blinded with secret
	return
}

// Decrypt performs the ElGamal decryption algorithm.
func Decrypt(private kyber.Scalar, K, C kyber.Point) kyber.Point {
	// ElGamal-decrypt the ciphertext (K,C) to reproduce the message.
	S := cothority.Suite.Point().Mul(private, K) // regenerate shared secret
	return cothority.Suite.Point().Sub(C, S)     // use to un-blind the message
}

// Verify performs verifies the proof of a Neff shuffle.
func Verify(tag []byte, public kyber.Point, x, y, v, w []kyber.Point) error {
	if len(x) < 2 || len(y) < 2 || len(v) < 2 || len(w) < 2 {
		return errors.New("cannot verify less than 2 points")
	}
	verifier := shuffle.Verifier(cothority.Suite, nil, public, x, y, v, w)
	return proof.HashVerify(cothority.Suite, "", verifier, tag)
}

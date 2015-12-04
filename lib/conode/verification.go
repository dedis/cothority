package conode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
)

/*
Verification methods used by stamper.
*/

// Verifies that the 'message' is included in the signature and that it
// is correct.
// Message is your own hash, and reply contains the inclusion proof + signature
// on the aggregated message
func VerifySignature(suite abstract.Suite, reply *StampSignature, public abstract.Point, message []byte) bool {
	// Check if aggregate public key is correct
	if !public.Equal(reply.AggPublic) {
		dbg.Lvl1("Aggregate-public-key check: FAILED (maybe you have an outdated config file of the tree)")
		return false
	}
	// First check if the challenge is ok
	if err := VerifyChallenge(suite, reply); err != nil {
		dbg.Lvl1("Challenge-check: FAILED (", err, ")")
		return false
	}
	dbg.Lvl2("Challenge-check: OK")

	// Incorporate the timestamp in the message since the verification process
	// is done by reconstructing the challenge
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, reply.Timestamp); err != nil {
		dbg.Lvl1("Error marshaling the timestamp for signature verification")
		return false
	}
	msg := append(b.Bytes(), []byte(reply.MerkleRoot)...)
	if err := VerifySchnorr(suite, msg, public, reply.Challenge, reply.Response); err != nil {
		dbg.Lvl1("Signature-check: FAILED (", err, ")")
		return false
	}
	dbg.Lvl2("Signature-check: OK")

	// finally check the proof
	if !proof.CheckProof(suite.Hash, reply.MerkleRoot, hashid.HashId(message), reply.Prf) {
		dbg.Lvl2("Inclusion-check: FAILED")
		return false
	}
	dbg.Lvl2("Inclusion-check: OK")
	return true
}

// verifyChallenge will reconstruct the challenge in order to see if any of the
// components of the challenge has been spoofed or not. It may be a different
// timestamp .
func VerifyChallenge(suite abstract.Suite, reply *StampSignature) error {
	dbg.Lvlf3("Reply is %+v", reply)
	// marshal the V
	pbuf, err := reply.AggCommit.MarshalBinary()
	if err != nil {
		return err
	}
	c := suite.Cipher(pbuf)
	// concat timestamp and merkle root
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, reply.Timestamp); err != nil {
		return err
	}
	cbuf := append(b.Bytes(), reply.MerkleRoot...)
	c.Message(nil, nil, cbuf)
	challenge := suite.Secret().Pick(c)
	if challenge.Equal(reply.Challenge) {
		return nil
	}
	return errors.New("Challenge reconstructed is not equal to the one given")
}

// A simple verification of a Schnorr signature given the message
func VerifySchnorr(suite abstract.Suite, message []byte, publicKey abstract.Point, c, r abstract.Secret) error {

	// Check that: base**r_hat * X_hat**c == V_hat
	// Equivalent to base**(r+xc) == base**(v) == T in vanillaElGamal
	Aux := suite.Point()
	V_clean := suite.Point()
	V_clean.Add(V_clean.Mul(nil, r), Aux.Mul(publicKey, c))
	// T is the recreated V_hat
	T := suite.Point().Null()
	T.Add(T, V_clean)

	// Verify that the hash based on the message and T
	// matches the challange c from the signature
	// copy of hashSchnorr
	bufPoint, _ := T.MarshalBinary()
	cipher := suite.Cipher(bufPoint)
	cipher.Message(nil, nil, message)
	hash := suite.Secret().Pick(cipher)
	if !hash.Equal(c) {
		return errors.New("invalid signature")
	}
	return nil
}

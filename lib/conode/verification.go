package conode
import (
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/hashid"
	"bytes"
	"encoding/binary"
	"github.com/dedis/cothority/lib/proof"
	"errors"
)

// Verifies that the 'message' is included in the signature and that it
// is correct.
// Message is your own hash, and reply contains the inclusion proof + signature
// on the aggregated message
func VerifySignature(suite abstract.Suite, reply *StampReply, public abstract.Point, message []byte) bool {
	// First check if the challenge is ok
	if err := VerifyChallenge(suite, reply); err != nil {
		dbg.Lvl1("Challenge-check : FAILED (", err, ")")
		return false
	}
	dbg.Lvl1("Challenge-check : OK")
	// Incorporate the timestamp in the message since the verification process
	// is done by reconstructing the challenge
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, reply.Timestamp); err != nil {
		dbg.Lvl1("Error marshaling the timestamp for signature verification")
	}
	msg := append(b.Bytes(), []byte(reply.MerkleRoot)...)
	if err := VerifySchnorr(suite, msg, public, reply.SigBroad.C, reply.SigBroad.R0_hat); err != nil {
		dbg.Lvl1("Signature-check : FAILED (", err, ")")
		return false
	}
	dbg.Lvl1("Signature-check : OK")

	// finally check the proof
	if !proof.CheckProof(suite.Hash, reply.MerkleRoot, hashid.HashId(message), reply.Prf) {
		dbg.Lvl1("Inclusion-check : FAILED")
		return false
	}
	dbg.Lvl1("Inclusion-check : OK")
	return true
}

// verifyChallenge will reconstruct the challenge in order to see if any of the
// components of the challenge has been spoofed or not. It may be a different
// timestamp .
func VerifyChallenge(suite abstract.Suite, reply *StampReply) error {

	// marshal the V
	pbuf, err := reply.SigBroad.V0_hat.MarshalBinary()
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
	if challenge.Equal(reply.SigBroad.C) {
		return nil
	}
	return errors.New("Challenge reconstructed is not equal to the one given ><")
}

// A simple verification of a schnorr signature given the message
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


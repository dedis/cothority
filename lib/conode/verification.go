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

	subPublic := suite.Point().Add(suite.Point().Null(), public)
	// Check if aggregate public key is correct
	for _, exception := range reply.RejectionPublicList {
		dbg.Lvlf4("Removing %v from public", exception)
		subPublic = subPublic.Sub(subPublic, exception)
	}

	if !subPublic.Equal(reply.AggPublic) {
		dbg.Lvl1("Aggregate-public-key check: FAILED (maybe you have an outdated config file of the tree)")
		return false
	}

	ExceptionV_hat := suite.Point().Null()
	for _, exception := range reply.RejectionCommitList {
		ExceptionV_hat = ExceptionV_hat.Add(ExceptionV_hat, exception)
	}

	V_clean := suite.Point()
	V_clean.Add(V_clean.Mul(nil, reply.Response), suite.Point().Mul(reply.AggPublic, reply.Challenge))
	// T is the recreated V_hat
	T := suite.Point().Null()
	T.Add(T, V_clean)
	T.Add(T, ExceptionV_hat)

	if !T.Equal(reply.AggCommit) {
		dbg.Lvl1("Aggregate Commit key check: FAILED")
		return false
	}
	dbg.Lvl1("Aggregate Commit key check : OK")
	var err error
	// First check if the challenge is ok
	if err = VerifyChallenge(suite, reply); err != nil {
		dbg.Lvl1("Challenge-check: FAILED (", err, ")")
		return false
	}
	dbg.Lvl2("Challenge-check: OK")

	// Incorporate the timestamp in the message since the verification process
	// is done by reconstructing the challenge
	var msg []byte
	if msg, err = concatTimeStampAndMerkleRoot(reply); err != nil {
		dbg.Lvl1("Could not concat from reply's timestamp and merkle root")
		return false
	}

	// Verify if base**R_hat * X_hat**C == base**(v_hat - x_hat*c + x_hat*c) == base**v_hat == V_hat
	// Here V_hat is the set of commitment used to SIGN, i.e. WITHOUT the nodes that refused to sign
	// Challenge is the challenge untouched
	// X_hat is the reduced set of aggregate public key, i.e. WITHOUT the node that refused to sign
	// R_hat is the reduced set of aggregate response, of course without the ones that did not want to sign ( == no response )
	if err := VerifySchnorr(suite, msg, subPublic, reply.Challenge, reply.Response, ExceptionV_hat); err != nil {
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

func concatTimeStampAndMerkleRoot(reply *StampSignature) ([]byte, error) {
	// concat timestamp and merkle root
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, reply.Timestamp); err != nil {
		return nil, err
	}
	cbuf := append(b.Bytes(), reply.MerkleRoot...)
	return cbuf, nil
}

// A simple verification of a Schnorr signature given the message
func VerifySchnorr(suite abstract.Suite, message []byte, publicKey abstract.Point, c, r abstract.Secret, allExceptionsSum abstract.Point) error {

	// Check that: base**r_hat * X_hat**c == V_hat
	// Equivalent to base**(r+xc) == base**(v) == T in vanillaElGamal
	Aux := suite.Point()
	V_clean := suite.Point()
	V_clean.Add(V_clean.Mul(nil, r), Aux.Mul(publicKey, c))
	// T is the recreated V_hat
	T := suite.Point().Null()
	// commitment from nodes that have signed.
	T = T.Add(T, V_clean)
	// We need to add the commitment keys from the ondes that did not sign also in
	// order to re-create the challenge (which is based on the full set of commitment keys)
	T = T.Add(T, allExceptionsSum)

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

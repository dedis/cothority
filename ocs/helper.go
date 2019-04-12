package ocs

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3/log"
)

// EncodeKey can be used by the writer to an onchain-secret skipchain
// to encode his symmetric key under the collective public key created
// by the DKG.
// As this method uses `Pick` to encode the key, depending on the key-length
// more than one point is needed to encode the data.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - key - the symmetric key for the document
//
// Output:
//   - U - the schnorr commit
//   - C - encrypted key
func EncodeKey(suite suites.Suite, X kyber.Point, key []byte) (U kyber.Point, C kyber.Point, err error) {
	if len(key) > suite.Point().EmbedLen() {
		return nil, nil, errors.New("got more data than can fit into one point")
	}
	r := suite.Scalar().Pick(suite.RandomStream())
	C = suite.Point().Mul(r, X)
	log.Lvl3("C:", C.String())
	U = suite.Point().Mul(r, nil)
	log.Lvl3("U is:", U.String())

	kp := suite.Point().Embed(key, suite.RandomStream())
	log.Lvl3("Keypoint:", kp.String())
	log.Lvl3("X:", X.String())
	C.Add(C, kp)
	return
}

// DecodeKey can be used by the reader of an onchain-secret to convert the
// re-encrypted secret back to a symmetric key that can be used later to
// decode the document.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - C - the encrypted key
//   - XhatEnc - the re-encrypted schnorr-commit
//   - xc - the private key of the reader
//
// Output:
//   - key - the re-assembled key
//   - err - an eventual error when trying to recover the data from the points
func DecodeKey(suite kyber.Group, X kyber.Point, C kyber.Point, XhatEnc kyber.Point,
	xc kyber.Scalar) (key []byte, err error) {
	log.Lvl3("xc:", xc)
	xcInv := suite.Scalar().Neg(xc)
	log.Lvl3("xcInv:", xcInv)
	sum := suite.Scalar().Add(xc, xcInv)
	log.Lvl3("xc + xcInv:", sum, "::", xc)
	log.Lvl3("X:", X)
	XhatDec := suite.Point().Mul(xcInv, X)
	log.Lvl3("XhatDec:", XhatDec)
	log.Lvl3("XhatEnc:", XhatEnc)
	Xhat := suite.Point().Add(XhatEnc, XhatDec)
	log.Lvl3("Xhat:", Xhat)
	XhatInv := suite.Point().Neg(Xhat)
	log.Lvl3("XhatInv:", XhatInv)

	// Decrypt C to keyPointHat
	log.Lvl3("C:", C)
	keyPointHat := suite.Point().Add(C, XhatInv)
	log.Lvl3("keyPointHat:", keyPointHat)
	key, err = keyPointHat.Data()
	if err != nil {
		return nil, Erret(err)
	}
	log.Lvl3("key:", key)
	return
}

func Erret(err error) error {
	if err == nil {
		return nil
	}
	pc, _, line, _ := runtime.Caller(1)
	errStr := err.Error()
	if strings.HasPrefix(errStr, "Erret") {
		errStr = "\n\t" + errStr
	}
	return fmt.Errorf("Erret at %s: %d -> %s", runtime.FuncForPC(pc).Name(), line, errStr)
}

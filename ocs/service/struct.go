package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"gopkg.in/satori/go.uuid.v1"
)

type suite interface {
	kyber.Group
	kyber.XOFFactory
}

// ServiceName is used for registration on the onet.
const ServiceName = "OnChainSecrets"

// VerifyOCS makes sure that all necessary signatures are present when
// updating the OCS-skipchain.
var VerifyOCS = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "OCS"))

// VerificationOCS adds the VerifyBase to the VerifyOCS for a complete
// skipchain.
var VerificationOCS = []skipchain.VerifierID{skipchain.VerifyBase,
	VerifyOCS}

// SkipChainURL represents a skipchain. It needs to know the roster of the
// responsible nodes, and the hash of the genesis-block, which is the ID
// of the Skipchain.
type SkipChainURL struct {
	Roster  *onet.Roster
	Genesis skipchain.SkipBlockID
}

// NewSkipChainURL returns a SkipChainURL from a skipblock.
func NewSkipChainURL(sb *skipchain.SkipBlock) *SkipChainURL {
	return &SkipChainURL{
		Roster:  sb.Roster,
		Genesis: sb.SkipChainID(),
	}
}

// NewOCS returns a pointer to a DataOCS structure created from
// the given data-slice. If the slice is not a valid DataOCS-structure,
// nil is returned.
func NewOCS(b []byte) *Transaction {
	dw := &Transaction{}
	err := protobuf.DecodeWithConstructors(b, dw, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		log.Error(err)
		return nil
	}
	return dw
}

// String returns a nice string.
func (dw *Transaction) String() string {
	if dw == nil {
		return "nil-pointer"
	}
	var str string
	if dw.Darc != nil {
		str += fmt.Sprintf("Darc: %s\n", dw.Darc)
	}
	if dw.Meta != nil {
		str += fmt.Sprintf("Meta: %x\n", *dw.Meta)
	}
	if dw.Write != nil {
		str += fmt.Sprintf("Write: data-length of %d\n", len(dw.Write.Data))
	}
	if dw.Read != nil {
		str += fmt.Sprintf("Read: %+v read data %x\n", dw.Read.Signature.SignaturePath.Signer, dw.Read.DataID)
	}
	return str
}

// NewWrite is used by the writer to an onchain-secret skipchain
// to encode his symmetric key under the collective public key created
// by the DKG.
// As this method uses `Embed` to encode the key, depending on the key-length
// more than one point is needed to encode the data.
//
// Input:
//   - suite - the cryptographic suite to use
//   - scid - the id of the skipchain - used to create the second generator
//   - X - the aggregate public key of the DKG
//   - reader - the darc that points to valid readers
//   - key - the symmetric key for the document
//
// Output:
//   - write - structure containing the encrypted key U, Cs and the NIZKP of
//   it containing the reader-darc.
func NewWrite(suite suites.Suite, scid skipchain.SkipBlockID, X kyber.Point, reader *darc.Darc, key []byte) *Write {
	wr := &Write{
		Reader: *reader,
	}
	r := suite.Scalar().Pick(suite.RandomStream())
	C := suite.Point().Mul(r, X)
	wr.U = suite.Point().Mul(r, nil)

	// Create proof
	for len(key) > 0 {
		kp := suite.Point().Embed(key, suite.RandomStream())
		wr.Cs = append(wr.Cs, suite.Point().Add(C, kp))
		key = key[min(len(key), kp.EmbedLen()):]
	}

	gBar := suite.Point().Mul(suite.Scalar().SetBytes(scid), nil)
	wr.Ubar = suite.Point().Mul(r, gBar)
	s := suite.Scalar().Pick(suite.RandomStream())
	w := suite.Point().Mul(s, nil)
	wBar := suite.Point().Mul(s, gBar)
	hash := sha256.New()
	for _, c := range wr.Cs {
		c.MarshalTo(hash)
	}
	wr.U.MarshalTo(hash)
	wr.Ubar.MarshalTo(hash)
	w.MarshalTo(hash)
	wBar.MarshalTo(hash)
	hash.Write(wr.Reader.GetID())
	wr.E = suite.Scalar().SetBytes(hash.Sum(nil))
	wr.F = suite.Scalar().Add(s, suite.Scalar().Mul(wr.E, r))
	return wr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CheckProof verifies that the write-request has actually been created with
// somebody having access to the secret key.
func (wr *Write) CheckProof(suite suite, scid skipchain.SkipBlockID) error {
	gf := suite.Point().Mul(wr.F, nil)
	ue := suite.Point().Mul(suite.Scalar().Neg(wr.E), wr.U)
	w := suite.Point().Add(gf, ue)

	gBar := suite.Point().Mul(suite.Scalar().SetBytes(scid), nil)
	gfBar := suite.Point().Mul(wr.F, gBar)
	ueBar := suite.Point().Mul(suite.Scalar().Neg(wr.E), wr.Ubar)
	wBar := suite.Point().Add(gfBar, ueBar)

	hash := sha256.New()
	for _, c := range wr.Cs {
		c.MarshalTo(hash)
	}
	wr.U.MarshalTo(hash)
	wr.Ubar.MarshalTo(hash)
	w.MarshalTo(hash)
	wBar.MarshalTo(hash)
	hash.Write(wr.Reader.GetID())
	e := suite.Scalar().SetBytes(hash.Sum(nil))
	if e.Equal(wr.E) {
		return nil
	}
	return errors.New("recreated proof is not equal to stored proof")
}

// DecodeKey can be used by the reader of an onchain-secret to convert the
// re-encrypted secret back to a symmetric key that can be used later to
// decode the document.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - Cs - the encrypted key-slices
//   - XhatEnc - the re-encrypted schnorr-commit
//   - xc - the private key of the reader
//
// Output:
//   - key - the re-assembled key
//   - err - an eventual error when trying to recover the data from the points
func DecodeKey(suite suite, X kyber.Point, Cs []kyber.Point, XhatEnc kyber.Point,
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

	// Decrypt Cs to keyPointHat
	for _, C := range Cs {
		log.Lvl3("C:", C)
		keyPointHat := suite.Point().Add(C, XhatInv)
		log.Lvl3("keyPointHat:", keyPointHat)
		keyPart, err := keyPointHat.Data()
		log.Lvl3("keyPart:", keyPart)
		if err != nil {
			return nil, err
		}
		key = append(key, keyPart...)
	}
	return
}

package calypso

import (
	"crypto/sha256"
	"fmt"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/kyber/v3/xof/keccak"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	network.RegisterMessages(CreateLTS{}, CreateLTSReply{},
		Authorize{}, AuthorizeReply{},
		DecryptKey{}, DecryptKeyReply{})
}

type suite interface {
	kyber.Group
	kyber.XOFFactory
}

// NewWrite is used by the writer to ByzCoin to encode his symmetric key
// under the collective public key created by the DKG.
//
// Input:
//   - suite - the cryptographic suite to use
//   - ltsid - the id of the LTS id - used to create the second generator
//   - writeDarc - the id of the darc where this write will be stored
//   - X - the aggregate public key of the DKG
//   - key - the symmetric key for the document - it will be encrypted in this
//   method
//
// Output:
//   - write - structure containing the encrypted key U, C and the NIZKP of
//   it containing the reader-darc. If it is nil then we failed to embed the
//   key because it is too long to represent the key using a point.
func NewWrite(suite suites.Suite, ltsid byzcoin.InstanceID, writeDarc darc.ID, X kyber.Point, key []byte) *Write {
	wr := &Write{LTSID: ltsid}
	r := suite.Scalar().Pick(suite.RandomStream())
	C := suite.Point().Mul(r, X)
	wr.U = suite.Point().Mul(r, nil)

	// Create proof
	if len(key) > suite.Point().EmbedLen() {
		return nil
	}
	kp := suite.Point().Embed(key, suite.RandomStream())
	wr.C = suite.Point().Add(C, kp)

	gBar := suite.Point().Embed(ltsid.Slice(), keccak.New(ltsid.Slice()))
	wr.Ubar = suite.Point().Mul(r, gBar)
	s := suite.Scalar().Pick(suite.RandomStream())
	w := suite.Point().Mul(s, nil)
	wBar := suite.Point().Mul(s, gBar)
	hash := sha256.New()
	wr.C.MarshalTo(hash)
	wr.U.MarshalTo(hash)
	wr.Ubar.MarshalTo(hash)
	w.MarshalTo(hash)
	wBar.MarshalTo(hash)
	hash.Write(writeDarc)
	wr.E = suite.Scalar().SetBytes(hash.Sum(nil))
	wr.F = suite.Scalar().Add(s, suite.Scalar().Mul(wr.E, r))
	return wr
}

// CheckProof verifies that the write-request has actually been created with
// somebody having access to the secret key.
func (wr *Write) CheckProof(suite suite, writeID darc.ID) error {
	gf := suite.Point().Mul(wr.F, nil)
	ue := suite.Point().Mul(suite.Scalar().Neg(wr.E), wr.U)
	w := suite.Point().Add(gf, ue)

	gBar := suite.Point().Embed(wr.LTSID.Slice(), keccak.New(wr.LTSID.Slice()))
	gfBar := suite.Point().Mul(wr.F, gBar)
	ueBar := suite.Point().Mul(suite.Scalar().Neg(wr.E), wr.Ubar)
	wBar := suite.Point().Add(gfBar, ueBar)

	hash := sha256.New()
	wr.C.MarshalTo(hash)
	wr.U.MarshalTo(hash)
	wr.Ubar.MarshalTo(hash)
	w.MarshalTo(hash)
	wBar.MarshalTo(hash)
	hash.Write(writeID)

	e := suite.Scalar().SetBytes(hash.Sum(nil))
	if e.Equal(wr.E) {
		return nil
	}

	return fmt.Errorf("recreated proof is not equal to stored proof:\n"+
		"%s\n%s", e.String(), wr.E.String())
}

type newLtsConfig struct {
	byzcoin.Proof
}

type reshareLtsConfig struct {
	byzcoin.Proof
	Commits  []kyber.Point
	OldNodes []kyber.Point
}

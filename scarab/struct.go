package scarab

import (
	"crypto/sha256"
	"errors"

	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

func init() {
	network.RegisterMessages(CreateLTS{}, CreateLTSReply{},
		DecryptKey{}, DecryptKeyReply{})
}

type suite interface {
	kyber.Group
	kyber.XOFFactory
}

// NewWrite is used by the writer to an onchain-secret skipchain
// to encode his symmetric key under the collective public key created
// by the DKG.
// As this method uses `Embed` to encode the key, depending on the key-length
// more than one point is needed to encode the data.
//
// Input:
//   - suite - the cryptographic suite to use
//   - ltsid - the id of the LTS id - used to create the second generator
//   - writeDarc - the id of the darc where this write will be stored
//   - X - the aggregate public key of the DKG
//   - key - the symmetric key for the document - it will be encrypted in this method
//
// Output:
//   - write - structure containing the encrypted key U, Cs and the NIZKP of
//   it containing the reader-darc.
func NewWrite(suite suites.Suite, ltsid []byte, writeDarc darc.ID, X kyber.Point, key []byte) *Write {
	wr := &Write{LTSID: ltsid}
	r := suite.Scalar().Pick(suite.RandomStream())
	C := suite.Point().Mul(r, X)
	wr.U = suite.Point().Mul(r, nil)

	// Create proof
	for len(key) > 0 {
		kp := suite.Point().Embed(key, suite.RandomStream())
		wr.Cs = append(wr.Cs, suite.Point().Add(C, kp))
		key = key[min(len(key), kp.EmbedLen()):]
	}

	gBar := suite.Point().Mul(suite.Scalar().SetBytes(ltsid), nil)
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
	hash.Write(writeDarc)
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
func (wr *Write) CheckProof(suite suite, writeID darc.ID) error {
	gf := suite.Point().Mul(wr.F, nil)
	ue := suite.Point().Mul(suite.Scalar().Neg(wr.E), wr.U)
	w := suite.Point().Add(gf, ue)

	gBar := suite.Point().Mul(suite.Scalar().SetBytes(wr.LTSID), nil)
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
	hash.Write(writeID)

	e := suite.Scalar().SetBytes(hash.Sum(nil))
	if e.Equal(wr.E) {
		return nil
	}

	return errors.New("recreated proof is not equal to stored proof")
}

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
//   - Cs - encrypted key-slices
func EncodeKey(suite suites.Suite, X kyber.Point, key []byte) (U kyber.Point, Cs []kyber.Point) {
	r := suite.Scalar().Pick(suite.RandomStream())
	C := suite.Point().Mul(r, X)
	log.Lvl3("C:", C.String())
	U = suite.Point().Mul(r, nil)
	log.Lvl3("U is:", U.String())

	for len(key) > 0 {
		var kp kyber.Point
		kp = suite.Point().Embed(key, suite.RandomStream())
		log.Lvl3("Keypoint:", kp.String())
		log.Lvl3("X:", X.String())
		Cs = append(Cs, suite.Point().Add(C, kp))
		log.Lvl3("Cs:", C.String())
		key = key[min(len(key), kp.EmbedLen()):]
	}
	return
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
func DecodeKey(suite kyber.Group, X kyber.Point, Cs []kyber.Point, XhatEnc kyber.Point,
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

// PROTOSTART
// import "skipblock.proto";
// import "darc.proto";
// import "roster.proto";
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "Scarab";

// ***
// Common structures
// ***

// Write is the data stored in a write instance. It stores a reference to the LTS
// used and the encrypted secret.
type Write struct {
	// Data should be encrypted by the application under the symmetric key in U and Cs
	Data []byte
	// U is the encrypted random value for the ElGamal encryption
	U kyber.Point
	// Ubar, E and f will be used by the server to verify the writer did
	// correctly encrypt the key. It binds the policy (the darc) with the
	// cyphertext.
	// Ubar is used for the log-equality proof
	Ubar kyber.Point
	// E is the non-interactive challenge as scalar
	E kyber.Scalar
	// f is the proof - written in uppercase here so it is an exported field,
	// but in the OCS-paper it's lowercase.
	F kyber.Scalar
	// Cs are the ElGamal parts for the symmetric key material (might
	// also contain an IV)
	Cs []kyber.Point
	// ExtraData is clear text and application-specific
	ExtraData *[]byte
	// LTSID points to the identity of the lts group
	LTSID []byte
}

// Read is the data stored in a read instance. It has a pointer to the write
// instance and the public key used to create the read instance.
type Read struct {
	Write ol.InstanceID
	Xc    kyber.Point
}

// ***
// These are the messages used in the API-calls
// ***

// CreateLTS is used to start a DKG and store the private keys in each node.
type CreateLTS struct {
	// Roster is the list of nodes that should participate in the DKG.
	Roster onet.Roster
	// OLID is the ID of the OmniLedger that can use this LTS.
	OLID skipchain.SkipBlockID
}

// CreateLTSReply is returned upon successfully setting up the distributed
// key.
type CreateLTSReply struct {
	// LTSID is a random 32-byte slice that represents the LTS.
	LTSID []byte
	// X is the public key of the LTS.
	X kyber.Point
	// TODO: can we remove the LTSID and only use the public key to identify
	// an LTS?
}

// DecryptKey is sent by a reader after he successfully stored a 'Read' request
// in omniledger.
type DecryptKey struct {
	// Read is the proof that he has been accepted to read the secret.
	Read ol.Proof
	// Write is the proof containing the write request.
	Write ol.Proof
}

// DecryptKeyReply is returned if the service verified successfully that the
// decryption request is valid.
type DecryptKeyReply struct {
	// Cs are the secrets re-encrypted under the reader's public key.
	Cs []kyber.Point
	// XhatEnc is the random part of the encryption.
	XhatEnc kyber.Point
	// X is the aggregate public key of the LTS used.
	X kyber.Point
}

// SharedPublicRequest asks for the shared public key of the corresponding
// LTSID
type SharedPublic struct {
	// LTSID is the id of the LTS instance created.
	LTSID []byte
}

// SharedPublicReply sends back the shared public key.
type SharedPublicReply struct {
	// X is the distributed public key.
	X kyber.Point
}

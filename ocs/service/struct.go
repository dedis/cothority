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

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		Transaction{}, Write{}, Read{},
		CreateSkipchainsRequest{}, CreateSkipchainsReply{},
		WriteRequest{}, WriteReply{},
		ReadRequest{}, ReadReply{},
		SharedPublicRequest{}, SharedPublicReply{},
		DecryptKeyRequest{}, DecryptKeyReply{},
		GetReadRequests{}, GetReadRequestsReply{})
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

	gBar := suite.Point().Pick(suite.XOF(scid))
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

	gBar := suite.Point().Pick(suite.XOF(scid))
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

// PROTOSTART
// import "skipblock.proto";
// import "darc.proto";
// import "roster.proto";
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "OCSProto";

// ***
// These are the messages used in the API-calls
// ***

// Transaction holds either:
// - a read request
// - a write
// - a key-update
// - a write and a key-update
// Additionally, it can hold a slice of bytes with any data that the user wants to
// add to bind to that transaction.
// Every Transaction must have a Unix timestamp.
type Transaction struct {
	// Write holds an eventual write-request with a document
	Write *Write
	// Read holds an eventual read-request, which is approved, for a document
	Read *Read
	// Darc defines either the readers allowed for this write-request
	// or is an update to an existing Darc
	Darc *darc.Darc
	// Meta is any free-form data in that skipblock
	Meta *[]byte
	// Unix timestamp to record the transaction creation time
	Timestamp int64
}

// Write stores the data and the encrypted secret
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
	// Reader points to a darc where the reading-rights are stored
	Reader darc.Darc
	// Signature must come from a valid writer stored in the admin darc of the OCS
	// skipchain. For backwards-compatibility, this is an optional field.
	// But for every new write-request, it must be set.
	Signature *darc.Signature
}

// Read stores a read-request which is the secret encrypted under the
// pseudonym's public key. The Data is the skipblock-id of the skipblock
// holding the data.
type Read struct {
	// DataID is the document-id for the read request
	DataID skipchain.SkipBlockID
	// Signature is a Schnorr-signature using the private key of the
	// reader on the message 'DataID'
	Signature darc.Signature
}

// ReadDoc represents one read-request by a reader.
type ReadDoc struct {
	Reader darc.Identity
	ReadID skipchain.SkipBlockID
	DataID skipchain.SkipBlockID
}

// ***
// Requests and replies to/from the service
// ***

// CreateSkipchainsRequest asks for setting up a new OCS-skipchain.
type CreateSkipchainsRequest struct {
	Roster  onet.Roster
	Writers darc.Darc
}

// CreateSkipchainsReply returns the skipchain-id of the OCS-skipchain
type CreateSkipchainsReply struct {
	OCS *skipchain.SkipBlock
	X   kyber.Point
}

// GetDarcPath returns the shortest path from the base darc to a darc
// containing the identity.
type GetDarcPath struct {
	OCS        skipchain.SkipBlockID
	BaseDarcID []byte
	Identity   darc.Identity
	Role       int
}

// GetDarcPathReply returns the shortest path to prove that the identity
// can sign. If there is no such path, Path is nil.
type GetDarcPathReply struct {
	Path *[]darc.Darc
}

// UpdateDarc allows to set up new accounts or edit existing
// read-rights in documents.
type UpdateDarc struct {
	OCS  skipchain.SkipBlockID
	Darc darc.Darc
}

// UpdateDarcReply contains the skipblock with the account stored
// in it. If the requested update is invalid, a nil skipblcok will
// be returned.
type UpdateDarcReply struct {
	SB *skipchain.SkipBlock
}

// WriteRequest asks the OCS-skipchain to store data on the skipchain.
// Readers can be empty if Write points to a valid reader that is already
// stored on the skipchain.
// The identity of the signature has to be a valid Writer-identity and
// must be the same as the publisher in the Write-request.
type WriteRequest struct {
	OCS       skipchain.SkipBlockID
	Write     Write
	Signature darc.Signature
	Readers   *darc.Darc
}

// WriteReply returns the created skipblock which is the write-id
type WriteReply struct {
	SB *skipchain.SkipBlock
}

// ReadRequest asks the OCS-skipchain to allow a reader to access a document.
type ReadRequest struct {
	OCS  skipchain.SkipBlockID
	Read Read
}

// ReadReply is the added skipblock, if successful.
type ReadReply struct {
	SB *skipchain.SkipBlock
}

// SharedPublicRequest asks for the shared public key of the corresponding
// skipchain-ID.
type SharedPublicRequest struct {
	Genesis skipchain.SkipBlockID
}

// SharedPublicReply sends back the shared public key.
type SharedPublicReply struct {
	X kyber.Point
}

// DecryptKeyRequest is sent to the service with the read-request. Optionally
// it can be given an Ephemeral public key under which the reply should be
// encrypted, but then a Signature on the key from the reader is needed.
type DecryptKeyRequest struct {
	Read skipchain.SkipBlockID
	// optional
	Ephemeral kyber.Point
	Signature *darc.Signature
}

// DecryptKeyReply is sent back to the api with the key encrypted under the
// reader's public key.
type DecryptKeyReply struct {
	Cs      []kyber.Point
	XhatEnc kyber.Point
	X       kyber.Point
}

// GetReadRequests asks for a list of requests
type GetReadRequests struct {
	Start skipchain.SkipBlockID
	Count int
}

// GetReadRequestsReply returns the requests
type GetReadRequestsReply struct {
	Documents []*ReadDoc
}

// GetBunchRequest asks for a list of bunches
type GetBunchRequest struct {
}

// GetBunchReply returns the genesis blocks of all registered OCS.
type GetBunchReply struct {
	Bunches []*skipchain.SkipBlock
}

// GetLatestDarc returns the path to the latest darc. DarcBaseID
// can be nil if DarcID has version==0.
type GetLatestDarc struct {
	OCS    skipchain.SkipBlockID
	DarcID []byte
}

// GetLatestDarcReply returns a list of all darcs, starting from
// the one requested. If the darc has not been found, it
// returns a nil list.
type GetLatestDarcReply struct {
	Darcs *[]*darc.Darc
}

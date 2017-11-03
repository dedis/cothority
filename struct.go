package ocs

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"fmt"

	"github.com/dedis/onchain-secrets/darc"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

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

const (
	// ErrorParameter is used when one of the parameters is faulty or leads
	// to a fault.
	ErrorParameter = iota + 4000
	// ErrorProtocol is used when one of the protocols (propagation) returns
	// an error.
	ErrorProtocol
)

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

// NewDataOCS returns a pointer to a DataOCS structure created from
// the given data-slice. If the slice is not a valid DataOCS-structure,
// nil is returned.
func NewDataOCS(b []byte) *Transaction {
	dw := &Transaction{}
	err := protobuf.DecodeWithConstructors(b, dw, network.DefaultConstructors(network.Suite))
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
	if dw.Write != nil {
		return fmt.Sprintf("Write: data-length of %d\n", len(dw.Write.Data))
	}
	if dw.Read != nil {
		return fmt.Sprintf("Read: %+v read data %x\n", dw.Read.Reader, dw.Read.DataID)
	}
	return "all nil DataOCS"
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
// additionally it can hold a slice of bytes with any data that the user wants to
// add to bind to that transaction.
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
}

// Write stores the data and the encrypted secret
type Write struct {
	// Data should be encrypted by the application under the symmetric key in U and Cs
	Data []byte
	// U is the encrypted random value for the ElGamal encryption
	U abstract.Point
	// Cs are the ElGamal parts for the symmetric key material (might
	// also contain an IV)
	Cs []abstract.Point
	// ExtraData is clear text and application-specific
	ExtraData *[]byte
	// Reader points to a darc where the reading-rights are stored
	Reader darc.Darc
}

// Read stores a read-request which is the secret encrypted under the
// pseudonym's public key. The Data is the skipblock-id of the skipblock
// holding the data.
type Read struct {
	// Reader represents the reader that signed the request
	Reader darc.Darc
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
	X   abstract.Point
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
	X abstract.Point
}

// DecryptKeyRequest is sent to the service with the read-request
type DecryptKeyRequest struct {
	Read skipchain.SkipBlockID
}

// DecryptKeyReply is sent back to the api with the key encrypted under the
// reader's public key.
type DecryptKeyReply struct {
	Cs      []abstract.Point
	XhatEnc abstract.Point
	X       abstract.Point
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

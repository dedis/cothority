package service

import (
	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
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

// PROTOSTART
// type :darc:darcOCS
// type :skipchain.SkipBlockID:bytes
// package ocs;
// import "skipchain.proto";
// import "darcOCS.proto";
// import "onet.proto";
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

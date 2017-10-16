package ocs

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"fmt"

	"github.com/satori/go.uuid"
	"github.com/dedis/cothority/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		DataOCS{}, DataOCSWrite{}, DataOCSRead{}, Darc{},
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

// DataOCS holds either:
// - a read request
// - a write
// - a key-update
// - a write and a key-update
// additionally it can hold a slice of bytes with any data that the user wants to
// add to bind to that transaction.
type DataOCS struct {
	Write   *DataOCSWrite
	Read    *DataOCSRead
	Readers *Darc
	Meta    *Meta
}

type Meta struct {
	Data []byte
}

// NewDataOCS returns a pointer to a DataOCS structure created from
// the given data-slice. If the slice is not a valid DataOCS-structure,
// nil is returned.
func NewDataOCS(b []byte) *DataOCS {
	_, dwi, err := network.Unmarshal(b)
	if err != nil {
		log.Error(err)
		return nil
	}
	if dwi == nil {
		log.Error("dwi is nil")
		return nil
	}
	dw, ok := dwi.(*DataOCS)
	if !ok {
		log.Error(err)
		return nil
	}
	return dw
}

// String returns a nice string.
func (dw *DataOCS) String() string {
	if dw == nil {
		return "nil-pointer"
	}
	if dw.Write != nil {
		return fmt.Sprintf("Write: data-length of %d", len(dw.Write.Data))
	}
	if dw.Read != nil {
		return fmt.Sprintf("Read: %s read data %x", dw.Read.Public, dw.Read.DataID)
	}
	return "all nil DataOCS"
}

// DataOCSWrite stores the data and the encrypted secret
type DataOCSWrite struct {
	Data []byte
	U    abstract.Point
	Cs   []abstract.Point
}

// DataOCSRead stores a read-request which is the secret encrypted under the
// pseudonym's public key. The Data is the skipblock-id of the skipblock
// holding the data.
type DataOCSRead struct {
	Public    abstract.Point
	DataID    skipchain.SkipBlockID
	Signature *crypto.SchnorrSig
}

// ReadDoc represents one read-request by a reader.
type ReadDoc struct {
	Reader abstract.Point
	ReadID skipchain.SkipBlockID
	DataID skipchain.SkipBlockID
}

// Requests and replies to/from the service

// CreateSkipchainsRequest asks for setting up a new OCS-skipchain.
type CreateSkipchainsRequest struct {
	Roster *onet.Roster
}

// CreateSkipchainsReply returns the skipchain-id of the OCS-skipchain
type CreateSkipchainsReply struct {
	OCS *skipchain.SkipBlock
	X   abstract.Point
}

// ReadDarcRequest returns the latest Darc for that ID. If recursive is
// true, it will search for all connected Darcs.
type ReadDarcRequest struct {
	OCS       skipchain.SkipBlockID
	DarcID    []byte
	Recursive bool
}

// ReadDarcReply returns all darcs that are attached to the requested
// darc. If a recursive search has been requested, the first darc will be
// the root-darc, but all other darcs will be in a random order.
type ReadDarcReply struct {
	Darc []*Darc
}

// EditDarcRequest allows to set up new accounts or edit existing
// read-rights in documents.
type EditDarcRequest struct {
	OCS  skipchain.SkipBlockID
	Darc *Darc
}

// EditDarcReply contains the skipblock with the account stored
// in it.
type EditDarcReply struct {
	SB *skipchain.SkipBlock
}

// WriteRequest asks the OCS-skipchain to store data on the skipchain.
// Readers can be empty if Write points to a valid reader.
type WriteRequest struct {
	OCS     skipchain.SkipBlockID
	Write   *DataOCSWrite
	Readers *Darc
	Data    *[]byte
}

// WriteReply returns the created skipblock which is the write-id
type WriteReply struct {
	SB *skipchain.SkipBlock
}

// ReadRequest asks the OCS-skipchain to allow a reader to access a document.
type ReadRequest struct {
	OCS  skipchain.SkipBlockID
	Read *DataOCSRead
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

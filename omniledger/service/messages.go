package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		&CreateGenesisBlock{}, &CreateGenesisBlockResponse{},
		&AddTxRequest{}, &AddTxResponse{},
	)
}

// Version indicates what version this client runs. In the first development
// phase, each next version will break the preceeding versions. Later on,
// new versions might correctly interpret earlier versions.
type Version int

// CurrentVersion is what we're running now
const CurrentVersion Version = 1

// PROTOSTART
// import "skipblock.proto";
// import "roster.proto";
// import "darc.proto";
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "OmniLedgerProto";

// ***
// These are the messages used in the API-calls
// ***

// CreateGenesisBlock asks the cisc-service to set up a new skipchain.
type CreateGenesisBlock struct {
	// Version of the protocol
	Version Version
	// Roster defines which nodes participate in the skipchain.
	Roster onet.Roster
	// GenesisDarc defines who is allowed to write to this skipchain.
	GenesisDarc darc.Darc
	// BlockInterval in int64.
	BlockInterval time.Duration
}

// CreateGenesisBlockResponse holds the genesis-block of the new skipchain.
type CreateGenesisBlockResponse struct {
	// Version of the protocol
	Version Version
	// Skipblock of the created skipchain or empty if there was an error.
	Skipblock *skipchain.SkipBlock
}

// AddTxRequest requests to apply a new transaction to the ledger.
type AddTxRequest struct {
	// Version of the protocol
	Version Version
	// SkipchainID is the hash of the first skipblock
	SkipchainID skipchain.SkipBlockID
	// Transaction to be applied to the kv-store
	Transaction ClientTransaction
}

// AddTxResponse is the reply after an AddTxRequest is finished.
type AddTxResponse struct {
	// Version of the protocol
	Version Version
}

// GetProof returns the proof that the given key is in the collection.
type GetProof struct {
	// Version of the protocol
	Version Version
	// Key is the key we want to look up
	Key []byte
	// ID is any block that is known to us in the skipchain, can be the genesis
	// block or any later block. The proof returned will be starting at this block.
	ID skipchain.SkipBlockID
}

// GetProofResponse can be used together with the Genesis block to proof that
// the returned key/value pair is in the collection.
type GetProofResponse struct {
	// Version of the protocol
	Version Version
	// Proof contains everything necessary to prove the inclusion
	// of the included key/value pair given a genesis skipblock.
	Proof Proof
}

// Config stores all the configuration information for one skipchain. It will
// be stored under the key "GenesisDarcID || OneNonce", in the collections. The
// GenesisDarcID is the value of GenesisReferenceID.
type Config struct {
	BlockInterval time.Duration
}

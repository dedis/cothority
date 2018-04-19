package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		&CreateSkipchain{}, &CreateSkipchainResponse{},
		&SetKeyValue{}, &SetKeyValueResponse{},
		&GetValue{}, &GetValueResponse{},
	)
}

const (
	// ErrorParse indicates an error while parsing the protobuf-file.
	ErrorParse = iota + 4000
)

// Version indicates what version this client runs. In the first development
// phase, each next version will break the preceeding versions. Later on,
// new versions might correctly interpret earlier versions.
type Version int

// CurrentVersion is what we're running now
const CurrentVersion Version = 1

// PROTOSTART
// import "skipblock.proto";
// import "roster.proto";
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "LleapProto";

// ***
// These are the messages used in the API-calls
// ***

// CreateSkipchain asks the cisc-service to set up a new skipchain.
type CreateSkipchain struct {
	// Version of the protocol
	Version Version
	// Roster defines which nodes participate in the skipchain.
	Roster onet.Roster
	// Transaction contains the master darc which defines who is allowed to
	// write to this skipchain. we will only store its hash.
	Transaction Transaction
}

// CreateSkipchainResponse holds the genesis-block of the new skipchain.
type CreateSkipchainResponse struct {
	// Version of the protocol
	Version Version
	// Skipblock of the created skipchain or empty if there was an error.
	Skipblock *skipchain.SkipBlock
}

// SetKeyValue asks for inclusion for a new key/value pair. The value needs
// to be signed by one of the Writers from the createSkipchain call.
type SetKeyValue struct {
	// Version of the protocol
	Version Version
	// SkipchainID is the hash of the first skipblock
	SkipchainID skipchain.SkipBlockID
	// Transaction to be apllied to the kv-store
	Transaction Transaction
}

// SetKeyValueResponse gives the timestamp and the skipblock-id
type SetKeyValueResponse struct {
	// Version of the protocol
	Version Version
	// Timestamp is milliseconds since the unix epoch (1/1/1970, 12am UTC)
	Timestamp *int64
	// Skipblock ID is the hash of the block where the value is stored
	SkipblockID *skipchain.SkipBlockID
}

// GetValue looks up the value in the given skipchain and returns the
// stored value, or an error if either the skipchain or the key doesn't exist.
type GetValue struct {
	// Version of the protocol
	Version Version
	// SkipchainID represents the skipchain where the value is stored
	SkipchainID skipchain.SkipBlockID
	// Key to retrieve
	Key  []byte
	Kind []byte
}

// GetValueResponse returns the value or an error if the key hasn't been found.
type GetValueResponse struct {
	//Version of the protocol
	Version Version
	// Value of the key
	Value *[]byte
	// Signature as sent when the value was stored
	Signature *[]byte
	// Proof the value is correct
	Proof *[]byte
}

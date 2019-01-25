package identity

import (
	"sync"

	"github.com/dedis/cothority/pop/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"go.dedis.ch/kyber"
)

// PROTOSTART
// type :map\[string\]Device:map<string, Device>
// type :map\[string\]string:map<string, string>
// type :map\[string\]\[\]byte:map<string, bytes>
// type :AuthType:sint32
// type :service.FinalStatement:pop.FinalStatement
// type :ID$:bytes
// package cisc;
// import "skipchain.proto";
// import "onet.proto";
// import "pop.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "CiscProto";

// IDBlock stores one identity together with the skipblocks.
type IDBlock struct {
	Latest          *Data
	Proposed        *Data
	LatestSkipblock *skipchain.SkipBlock
	sync.Mutex
}

// Data holds the information about all devices and the data stored in this
// identity-blockchain. All Devices have voting-rights to the Data-structure.
type Data struct {
	// Threshold of how many devices need to sign to accept the new block
	Threshold int
	// Device is a list of all devices allowed to sign
	Device map[string]*Device
	// Storage is the key/value storage
	Storage map[string]string
	// Roster is the new proposed roster - nil if the old is to be used
	Roster *onet.Roster
	// Votes for that block, mapped by name of the devices.
	// This has to be verified with the previous data-block, because only
	// the previous data-block has the authority to sign for a new block.
	Votes map[string][]byte
}

// Device is represented by a public key.
type Device struct {
	// Point is the public key of that device
	Point kyber.Point
}

// ***
// These are the messages used in the API-calls
// ***

// PinRequest used for admin autentification
type PinRequest struct {
	PIN    string
	Public kyber.Point
}

// StoreKeys used for setting autentification
type StoreKeys struct {
	Type    AuthType
	Final   *service.FinalStatement
	Publics []kyber.Point
	Sig     []byte
}

// CreateIdentity starts a new identity-skipchain with the initial
// Data and asking all nodes in Roster to participate.
type CreateIdentity struct {
	// Data is the first data that will be stored in the genesis-block. It should
	// contain the roster and at least one public key
	Data *Data
	// What type of authentication we're doing
	Type AuthType
	// SchnSig is optional; one of Public or SchnSig must be set.
	SchnSig *[]byte
	// authentication via Linkable Ring Signature
	Sig []byte
	// Nonce plays in this case message of authentication
	Nonce []byte
}

// CreateIdentityReply is the reply when a new Identity has been added. It
// returns the Root and Data-skipchain.
type CreateIdentityReply struct {
	Genesis *skipchain.SkipBlock
}

// DataUpdate verifies if a new update is available.
type DataUpdate struct {
	ID ID
}

// DataUpdateReply returns the updated data.
type DataUpdateReply struct {
	Data *Data
}

// ProposeSend sends a new proposition to be stored in all identities. It
// either replies a nil-message for success or an error.
type ProposeSend struct {
	ID      ID
	Propose *Data
}

// ProposeUpdate verifies if new data is available.
type ProposeUpdate struct {
	ID ID
}

// ProposeUpdateReply returns the updated propose-data.
type ProposeUpdateReply struct {
	Propose *Data
}

// ProposeVote sends the signature for a specific IdentityList. It replies nil
// if the threshold hasn't been reached, or the new SkipBlock
type ProposeVote struct {
	ID        ID
	Signer    string
	Signature []byte
}

// ProposeVoteReply returns the signed new skipblock if the threshold of
// votes have arrived.
type ProposeVoteReply struct {
	Data *skipchain.SkipBlock
}

// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	IDBlock *IDBlock
	Tag     string
	PubStr  string
}

// UpdateSkipBlock asks the service to fetch the latest SkipBlock
type UpdateSkipBlock struct {
	ID     ID
	Latest *skipchain.SkipBlock
}

// Authenticate first message of authentication protocol
// Empty message serves as trigger to start authentication protocol
// It also serves as response from server to sign nonce within LinkCtx
type Authenticate struct {
	Nonce []byte
	Ctx   []byte
}

package identity

import (
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/cothority/pop/service"
)

// PROTOSTART
// Messages between the Client-API and the Service

// Device is represented by a public key.
type Device struct {
	// Point is the public key of that device
	Point kyber.Point
}

// !!! Definitions of SchorrSig and ID structs cannot be found in any go file. !!!

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
	*IDBlock
	Tag    string
	PubStr string
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


package sidentity

import (
	//"encoding/binary"
	//"fmt"
	//"sort"
	//"strings"

	"github.com/dedis/cothority/crypto"
	//"github.com/dedis/cothority/log"
	//"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000

// ID represents one skipblock and corresponds to its Hash.
type ID skipchain.SkipBlockID

type PinState struct {
	// The type of our identity ("device", "ws", "client" or "client_with_pins")
	Ctype string
	// Minimum number of 'Pins' keys signing the new skipblock
	Threshold int
	// The trusted pins for the time interval 'Window'
	Pins []abstract.Point
	// Trusted window in seconds for the current 'Pins'
	Window int64
}

func NewPinState(ctype string, threshold int, pins []abstract.Point, window int64) *PinState {
	return &PinState{
		Ctype:     ctype,
		Threshold: threshold,
		Pins:      pins,
		Window:    window,
	}
}

// Messages between the Client-API and the Service

// CreateIdentity starts a new identity-skipchain with the initial
// Config and asking all nodes in Roster to participate.
type CreateIdentity struct {
	Config *common_structs.Config
	Roster *sda.Roster
}

// CreateIdentityReply is the reply when a new Identity has been added. It
// returns the Root and Data-skipchain.
type CreateIdentityReply struct {
	Root *skipchain.SkipBlock
	Data *skipchain.SkipBlock
}

// ConfigUpdate verifies if a new update is available.
type ConfigUpdate struct {
	ID skipchain.SkipBlockID // the Hash of the genesis skipblock
}

// ConfigUpdateReply returns the updated configuration.
type ConfigUpdateReply struct {
	Config *common_structs.Config
}

// GetUpdateChain - the client sends the hash of the last known
// Skipblock and will get back a list of all necessary SkipBlocks
// to get to the latest.
type GetUpdateChain struct {
	LatestID skipchain.SkipBlockID
	ID       skipchain.SkipBlockID
}

// GetUpdateChainReply - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainReply struct {
	Update []*skipchain.SkipBlock
}

// ProposeSend sends a new proposition to be stored in all identities. It
// either replies a nil-message for success or an error.
type ProposeSend struct {
	ID skipchain.SkipBlockID
	*common_structs.Config
}

// ProposeUpdate verifies if a new config is available.
type ProposeUpdate struct {
	ID skipchain.SkipBlockID
}

// ProposeUpdateReply returns the updated propose-configuration.
type ProposeUpdateReply struct {
	Propose *common_structs.Config
}

// ProposeVote sends the signature for a specific IdentityList. It replies nil
// if the threshold hasn't been reached, or the new SkipBlock
type ProposeVote struct {
	ID        skipchain.SkipBlockID
	Signer    string
	Signature *crypto.SchnorrSig
}

// ProposeVoteReply returns the signed new skipblock if the threshold of
// votes have arrived.
type ProposeVoteReply struct {
	Data *skipchain.SkipBlock
}

// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	*Storage
}

// UpdateSkipBlock asks the service to fetch the latest SkipBlock
type UpdateSkipBlock struct {
	ID       skipchain.SkipBlockID
	Latest   *skipchain.SkipBlock
	Previous *skipchain.SkipBlock
}

package identity

import (
	"github.com/dedis/cothority/skipchain"
	"github.com/satori/go.uuid"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	for _, s := range []interface{}{
		// API messages
		&CreateIdentity{},
		&CreateIdentityReply{},
		&ProposeSend{},
		&ProposeUpdate{},
		&ProposeUpdateReply{},
		&ProposeVote{},
		&ProposeVoteReply{},
	} {
		network.RegisterMessage(s)
	}
}

var VerifyIdentity = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "Identity"))
var VerificationIdentity = []skipchain.VerifierID{skipchain.VerifyBase,
	VerifyIdentity}

// Messages between the Client-API and the Service

// CreateIdentity takes a configuration and a genesis-skipblock. It adds
// the new
type CreateIdentity struct {
	Roster *onet.Roster
	Config *Config
}

// CreateIdentityReply is the reply when a new Identity has been added. It
// returns the genesis-skipblock.
type CreateIdentityReply struct {
	Genesis *skipchain.SkipBlock
}

// ProposeSend sends a new proposition to be stored in all identities. It
// either replies a nil-message for success or an error.
type ProposeSend struct {
	ID      skipchain.SkipBlockID
	Propose *Config
}

// ProposeUpdate verifies if a new config is available.
type ProposeUpdate struct {
	ID skipchain.SkipBlockID
}

// ProposeUpdateReply returns the updated propose-configuration.
type ProposeUpdateReply struct {
	Propose *Config
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

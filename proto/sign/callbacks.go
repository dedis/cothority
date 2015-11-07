package sign
import (
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/hashid"
)

// Callbacks holds the functions that are used to define the
// behaviour of a peer. All different peer-types use the
// cothority-tree, but they can interact differently with
// each other
type Callbacks interface {
	// Announcement: root -> nodes
	// This is called from the root-node whenever an
	// announcement is made.
	// TODO: remove Node-argument from function - this should be kept as
	// internal variable in CallbackStamper
	Announcement(*Node, *AnnouncementMessage) ([]*AnnouncementMessage, error)
	// Commitment: nodes -> root
	// This is called whenever a commitment is ready to
	// be sent. It takes the messages of its children and returns
	// the new message to be sent.
	///Commitment([]CommitmentMessage) *CommitmentMessage
	// Actual Commitment which only returns new Merkle-tree
	Commitment([]*CommitmentMessage) *CommitmentMessage
	// Challenge: root -> nodes
	// This is called with the message to be signed. If necessary,
	// each node can change the message for its children.
	Challenge(*ChallengeMessage)
	// Response: nodes -> root
	// This is called with the signature of the challenge-message
	// or with updated ExceptionList* in case of refusal to sign.
	Response(*ResponseMessage)
	// SignatureBroadcast: root -> nodes
	// This is called whenever the turn is completed and
	// the results are propagated through the tree.
	SignatureBroadcast(view int, SNRoot hashid.HashId, LogHash hashid.HashId, pr proof.Proof,
	sb *SignatureBroadcastMessage, suite abstract.Suite)
	// Statistics: nodes -> root
	// This is called at the end to collect eventual statistics
	// about the round.

	// Setup can be used to start listening on a port for requests or
	// any other setup that needs to be done
	Setup(*Peer) error
}


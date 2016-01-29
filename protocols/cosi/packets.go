package cosi

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

// This files defines the structure we use for registering to the channels by
// SDA.

// The main messages used by CoSi

// Broadcasted message initiated and signed by proposer
type AnnouncementMessage struct {
	// From = TreeNodeId in the Tree
	From      uuid.UUID
	Message   []byte
	RoundType string // what kind of round this announcement is made for
}

// Commitment of all nodes together with the data they want
// to have signed
type CommitmentMessage struct {

	// From = TreeNodeId in the Tree
	From    uuid.UUID
	Message []byte
	V       abstract.Point // commitment Point
	V_hat   abstract.Point // product of subtree participating nodes' commitment points
	X_hat   abstract.Point // product of subtree participating nodes' public keys

	MTRoot hashid.HashId // root of Merkle (sub)Tree

	// public keys of children servers that did not respond to
	// annoucement from root
	RejectionPublicList []abstract.Point

	Messages int // Actual number of messages signed
}

// The challenge calculated by the root-node
type ChallengeMessage struct {
	// From = TreeNodeId in the Tree
	From    uuid.UUID
	Message []byte
	C       abstract.Secret // challenge

	// Depth  byte
	MTRoot hashid.HashId // the very root of the big Merkle Tree
	Proof  proof.Proof   // Merkle Path of Proofs from root to us
}

// Every node replies with eventual exceptions if they
// are not OK
type ResponseMessage struct {
	// From = TreeNodeId in the Tree
	From    uuid.UUID
	Message []byte
	R_hat   abstract.Secret // response

	// public keys of children servers that did not respond to
	// challenge from root
	RejectionPublicList []abstract.Point
	// nodes that refused to commit:
	RejectionCommitList []abstract.Point

	// cummulative point commits of nodes that failed after commit
	ExceptionV_hat abstract.Point
	// cummulative public keys of nodes that failed after commit
	ExceptionX_hat abstract.Point
}

//Theses are pairs of TreeNode + the actual message we want to listen on.
type chanAnnouncement struct {
	*sda.TreeNode
	AnnouncementMessage
}

type chanCommitment struct {
	*sda.TreeNode
	CommitmentMessage
}

type chanChallenge struct {
	*sda.TreeNode
	ChallengeMessage
}

type chanResponse struct {
	*sda.TreeNode
	ResponseMessage
}

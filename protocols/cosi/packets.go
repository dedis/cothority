package cosi

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
)

// This files defines the structure we use for registering to the channels by
// SDA.

// The main messages used by CoSi

// Broadcasted message initiated and signed by proposer
type CosiAnnouncement struct {
	// From = TreeNodeId in the Tree
	From uuid.UUID
	*cosi.Announcement
}

// Commitment of all nodes together with the data they want
// to have signed
type CosiCommitment struct {
	*cosi.Commitment
}

/* Message []byte*/
//V       abstract.Point // commitment Point
//V_hat   abstract.Point // product of subtree participating nodes' commitment points
//X_hat   abstract.Point // product of subtree participating nodes' public keys

//MTRoot hashid.HashId // root of Merkle (sub)Tree

//// public keys of children servers that did not respond to
//// annoucement from root
//RejectionPublicList []abstract.Point

//Messages int // Actual number of messages signed
//}

// The challenge calculated by the root-node
type CosiChallenge struct {
	*cosi.Challenge
}

/* Message []byte*/
//C       abstract.Secret // challenge

//// Depth  byte
//MTRoot hashid.HashId // the very root of the big Merkle Tree
//Proof  proof.Proof   // Merkle Path of Proofs from root to us
//}

// Every node replies with eventual exceptions if they
// are not OK
type CosiResponse struct {
	*cosi.Response
}

/* Message []byte*/
//R_hat   abstract.Secret // response

//// public keys of children servers that did not respond to
//// challenge from root
//RejectionPublicList []abstract.Point
//// nodes that refused to commit:
//RejectionCommitList []abstract.Point

//// cummulative point commits of nodes that failed after commit
//ExceptionV_hat abstract.Point
//// cummulative public keys of nodes that failed after commit
//ExceptionX_hat abstract.Point
//}

//Theses are pairs of TreeNode + the actual message we want to listen on.
type chanAnnouncement struct {
	*sda.TreeNode
	CosiAnnouncement
}

type chanCommitment struct {
	*sda.TreeNode
	CosiCommitment
}

type chanChallenge struct {
	*sda.TreeNode
	CosiChallenge
}

type chanResponse struct {
	*sda.TreeNode
	CosiResponse
}

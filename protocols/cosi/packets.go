package cosi

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/sda"
)

// This files defines the structure we use for registering to the channels by
// SDA.

// The main messages used by CoSi

// see https://github.com/dedis/cothority/issues/260
// 0 - no check at all
// 1 - check only at root
// 2 - check at each level of the tree
var VerifyResponse = 1

// Announcement is broadcasted message initiated and signed by proposer.
type Announcement struct {
	// From = TreeNodeId in the Tree
	From sda.TreeNodeID
	*cosi.Announcement
}

// Commitment of all nodes together with the data they want
// to have signed
type Commitment struct {
	*cosi.Commitment
}

// XXX add the exception mechanism:
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

// Challenge is the challenge computed by the root-node.
type Challenge struct {
	*cosi.Challenge
}

/* Message []byte*/
//C       abstract.Secret // challenge

//// Depth  byte
//MTRoot hashid.HashId // the very root of the big Merkle Tree
//Proof  proof.Proof   // Merkle Path of Proofs from root to us
//}

// Response with which every node replies with.
// XXX currently without exceptions (even if they occur)
type Response struct {
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
	Announcement
}

type chanCommitment struct {
	*sda.TreeNode
	Commitment
}

type chanChallenge struct {
	*sda.TreeNode
	Challenge
}

type chanResponse struct {
	*sda.TreeNode
	Response
}

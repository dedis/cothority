package swupdate

import (
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// The main messages used by CoSi

// Announcement is broadcasted message initiated and signed by proposer.
type Announcement struct {
	Data []byte
}

// Commitment of all nodes together with the data they want
// to have signed
type Commitment struct {
	Comm abstract.Point
	// index of nodes that don't sign in this round
	// The index is taken from TreeNodeInstance.Index()
	RefusingNodes []uint32
}

// Challenge is the challenge computed by the root-node.
type Challenge struct {
	Chall abstract.Scalar
}

// Response with which every node replies with.
type Response struct {
	Resp abstract.Scalar
}

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

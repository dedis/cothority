package protocol

import (
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// This files defines the structure we use for registering to the channels by
// SDA.

// The main messages used by CoSi

// Announcement is broadcasted message initiated and signed by proposer.
type Announcement struct {
}

// Commitment of all nodes together with the data they want
// to have signed
type Commitment struct {
	Comm abstract.Point
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

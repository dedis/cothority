package swupdate

import (
	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/network"
)

// The main messages used by CoSi

func init(){
	for _, m := range []interface{}{Announcement{}, Commitment{}, Challenge{}, Response{}}{
		network.RegisterMessage(m)
	}
}

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
	*onet.TreeNode
	Announcement
}

type chanCommitment struct {
	*onet.TreeNode
	Commitment
}

type chanChallenge struct {
	*onet.TreeNode
	Challenge
}

type chanResponse struct {
	*onet.TreeNode
	Response
}

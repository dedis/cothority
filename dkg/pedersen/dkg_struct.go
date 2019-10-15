package pedersen

import (
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/network"

	dkgpedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
)

func init() {
	network.RegisterMessages(&SharedSecret{},
		&Init{}, &InitReply{},
		&StartDeal{}, &Deal{})
}

// SharedSecret represents the needed information to do shared encryption
// and decryption.
type SharedSecret struct {
	Index   int
	V       kyber.Scalar
	X       kyber.Point
	Commits []kyber.Point
}

// Clone makes a clone of the shared secret.
func (ss *SharedSecret) Clone() *SharedSecret {
	commits := make([]kyber.Point, len(ss.Commits))
	for i := range ss.Commits {
		commits[i] = ss.Commits[i].Clone()
	}
	return &SharedSecret{
		Index:   ss.Index,
		V:       ss.V.Clone(),
		X:       ss.X.Clone(),
		Commits: commits,
	}
}

// Init asks all nodes to set up a private/public key pair. It is sent to
// all nodes from the root-node. If Wait is true, at the end of the setup
// an additional message is sent to wait for all nodes to be set up.
type Init struct {
	Wait bool
}

type structInit struct {
	*onet.TreeNode
	Init
}

// InitReply returns the public key of that node.
type InitReply struct {
	Public kyber.Point
}

type structInitReply struct {
	*onet.TreeNode
	InitReply
}

// StartDeal is used by the leader to initiate the Deals.
type StartDeal struct {
	Publics   []kyber.Point
	Threshold uint32
}

type structStartDeal struct {
	*onet.TreeNode
	StartDeal
}

// Deal sends the deals for the shared secret.
type Deal struct {
	Deal *dkgpedersen.Deal
}

type structDeal struct {
	*onet.TreeNode
	Deal
}

// Response is sent to all other nodes.
type Response struct {
	Response *dkgpedersen.Response
}

type structResponse struct {
	*onet.TreeNode
	Response
}

// WaitSetup is only sent if Init.Wait == true
type WaitSetup struct {
}

type structWaitSetup struct {
	*onet.TreeNode
	WaitSetup
}

// WaitReply is sent once everything is set up
type WaitReply struct{}

type structWaitReply struct {
	*onet.TreeNode
	WaitReply
}

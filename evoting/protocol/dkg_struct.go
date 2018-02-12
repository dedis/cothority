package protocol

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share/dkg/rabin"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

func init() {
	network.RegisterMessages(
		&Init{}, &InitReply{},
		&StartDeal{}, &Deal{},
		&Response{}, &SecretCommit{},
		&Verification{}, &VerificationReply{})
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
	Deal *dkg.Deal
}

type structDeal struct {
	*onet.TreeNode
	Deal
}

// Response is sent to all other nodes.
type Response struct {
	Response *dkg.Response
}

type structResponse struct {
	*onet.TreeNode
	Response
}

// SecretCommit is sent to all other nodes.
type SecretCommit struct {
	SecretCommit *dkg.SecretCommits
}

type structSecretCommit struct {
	*onet.TreeNode
	SecretCommit
}

// Verification asks all nodes to verify the completion of the
// protocol and to return the collective public key.
type Verification struct {
}

type structVerification struct {
	*onet.TreeNode
	Verification
}

// VerificationReply contains the public key or nil if the verification failed
type VerificationReply struct {
	Public kyber.Point
}

type structVerificationReply struct {
	*onet.TreeNode
	VerificationReply
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

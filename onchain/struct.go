package onchain

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/share/dkg"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/network"
)

// Name can be used from other packages to refer to this protocol.
const Name = "OnchainSecret"

func init() {
	network.RegisterMessages(&Init{}, &InitReply{},
		&StartDeal{}, &Deal{},
		&Response{}, &SecretCommit{},
		&Verification{}, &VerificationReply{})
}

// Init asks all nodes to set up a private/public key pair. It is sent to
// the next node in the
type Init struct {
}

type chanInit struct {
	*onet.TreeNode
	Init
}

// InitReply returns the public key of that node.
type InitReply struct {
	Public abstract.Point
}

type chanInitReply struct {
	*onet.TreeNode
	InitReply
}

// StartDeal is used by the leader to initiate the Deals.
type StartDeal struct {
	Publics   []abstract.Point
	Threshold uint
}

type chanStartDeal struct {
	*onet.TreeNode
	StartDeal
}

// Deal sends the deals for the shared secret.
type Deal struct {
	Deal *dkg.Deal
}

type chanDeal struct {
	*onet.TreeNode
	Deal
}

// Response is sent to all other nodes.
type Response struct {
	Response *dkg.Response
}

type chanResponse struct {
	*onet.TreeNode
	Response
}

// SecretCommit is sent to all other nodes.
type SecretCommit struct {
	SecretCommit *dkg.SecretCommits
}

type chanSecretCommit struct {
	*onet.TreeNode
	SecretCommit
}

// Verification asks all nodes to verify the completion of the
// protocol and to return the collective public key.
type Verification struct {
}

type chanVerification struct {
	*onet.TreeNode
	Verification
}

// VerificationReply contains the public key or nil if the
// verification failed
type VerificationReply struct {
	Public abstract.Point
}

type chanVerificationReply struct {
	*onet.TreeNode
	VerificationReply
}

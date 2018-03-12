package protocol

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"errors"

	"github.com/dedis/kyber"
	dkg "github.com/dedis/kyber/share/dkg/rabin"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// NameDKG can be used from other packages to refer to this protocol.
const NameDKG = "SetupDKG"

func init() {
	network.RegisterMessages(&SharedSecret{},
		&Init{}, &InitReply{},
		&StartDeal{}, &Deal{},
		&Response{}, &SecretCommit{},
		&Verification{}, &VerificationReply{})
}

// SharedSecret represents the needed information to do shared encryption
// and decryption.
type SharedSecret struct {
	Index   int
	V       kyber.Scalar
	X       kyber.Point
	Commits []kyber.Point
}

// NewSharedSecret takes an initialized DistKeyGenerator and returns the
// minimal set of values necessary to do shared encryption/decryption.
func NewSharedSecret(dkg *dkg.DistKeyGenerator) (*SharedSecret, error) {
	if dkg == nil {
		return nil, errors.New("no valid dkg given")
	}
	if !dkg.Finished() {
		return nil, errors.New("dkg is not finished yet")
	}
	dks, err := dkg.DistKeyShare()
	if err != nil {
		return nil, err
	}
	return &SharedSecret{
		Index:   dks.Share.I,
		V:       dks.Share.V,
		X:       dks.Public(),
		Commits: dks.Commits,
	}, nil
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

// VerificationReply contains the public key or nil if the
// verification failed
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

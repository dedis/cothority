package protocol

/*
OCS_struct holds all messages for the onchain-secret protocol.
*/

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// NameOCS can be used from other packages to refer to this protocol.
const NameOCS = "OCS"

func init() {
	network.RegisterMessages(&Reencrypt{}, &ReencryptReply{})
}

// VerifyRequest is a callback-function that can be set by a service.
// Whenever a reencryption request is received, this function will be
// called and its return-value used to determine whether or not to
// allow reencryption.
type VerifyRequest func(rc *Reencrypt) bool

// Reencrypt asks for a re-encryption share from a node
type Reencrypt struct {
	U                kyber.Point
	Xc               kyber.Point
	VerificationData *[]byte
}

type structReencrypt struct {
	*onet.TreeNode
	Reencrypt
}

// ReencryptReply returns the share to re-encrypt from one node
type ReencryptReply struct {
	Ui *share.PubShare
	Ei kyber.Scalar
	Fi kyber.Scalar
}

type structReencryptReply struct {
	*onet.TreeNode
	ReencryptReply
}

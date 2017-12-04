package protocol

/*
OCS_struct holds all messages for the onchain-secret protocol.
*/

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/network"
)

// NameOCS can be used from other packages to refer to this protocol.
const NameOCS = "OCS"

func init() {
	network.RegisterMessages(&Reencrypt{}, &ReencryptReply{})
}

// Reencrypt asks for a re-encryption share from a node
type Reencrypt struct {
	U  abstract.Point
	Xc abstract.Point
}

type structReencrypt struct {
	*onet.TreeNode
	Reencrypt
}

// ReencryptReply returns the share to re-encrypt from one node
type ReencryptReply struct {
	Ui *share.PubShare
	Ei abstract.Scalar
	Fi abstract.Scalar
}

type structReencryptReply struct {
	*onet.TreeNode
	ReencryptReply
}

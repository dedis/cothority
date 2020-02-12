package ocsnt

import (
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

/*
OCSNT_struct holds all messages for the onchain-secret protocol.
*/

// NameOCSNT can be used from other packages to refer to this protocol.
const NameOCSNT = "OCSNT"

func init() {
	//network.RegisterMessages(&PartialReencrypt{}, &PartialReencryptReply{}, &ReadyReply{})
	network.RegisterMessages(&StartReencrypt{}, &PartialReencryption{}, &Ready{})
}

// VerifyRequest is a callback-function that can be set by a service.
// Whenever a reencryption request is received, this function will be
// called and its return-value used to determine whether or not to
// allow reencryption.
type VerifyPartialRequest func(sr *StartReencrypt) bool

// PartialReencrypt asks for a re-encryption share from a node
//type PartialReencrypt struct {
type StartReencrypt struct {
	IsReenc bool
	DKID    string
	// U is the point from the write-request
	U kyber.Point
	// Xc is the public key of the reader
	Xc kyber.Point
	// VerificationData is optional and can be any slice of bytes, so that each
	// node can verify if the reencryption request is valid or not.
	VerificationData *[]byte
	Pr               PartialReencryption
}

type structStartReencrypt struct {
	*onet.TreeNode
	StartReencrypt
}

type PartialReencryption struct {
	Ui *share.PubShare
	Ei kyber.Scalar
	Fi kyber.Scalar
}

type structPartialReencryption struct {
	*onet.TreeNode
	PartialReencryption
}

type structReady struct {
	*onet.TreeNode
	Ready
}

type Ready struct {
	Success bool
}

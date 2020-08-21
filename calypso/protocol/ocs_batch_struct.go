package protocol

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

const NameOCSBatch = "OCSBatch"

func init() {
	network.RegisterMessages(&RCBatch{}, &RCBReply{})
}

type VerifyBatchRequest func(rcd *RCData) bool

type RCInput struct {
	//Shared           *dkgprotocol.SharedSecret // Shared represents the private key
	Poly             *share.PubPoly // Represents all public keys
	U                kyber.Point    // U is the encrypted secret
	Xc               kyber.Point    // The client's public key
	VerificationData []byte
}

///////////////////////////////

type RCData struct {
	//Shared           *dkgprotocol.SharedSecret
	U                kyber.Point
	Xc               kyber.Point
	VerificationData *[]byte
}
type RCBatch struct {
	//RC []RCData
	RC map[int]*RCData
}

type structRCBatch struct {
	*onet.TreeNode
	RCBatch
}

///////////////////////////////

type RCReply struct {
	Ui *share.PubShare
	Ei kyber.Scalar
	Fi kyber.Scalar
}

type RCBReply struct {
	//RCR []RCReply
	RCR map[int]*RCReply
}

type structRCBReply struct {
	*onet.TreeNode
	RCBReply
}

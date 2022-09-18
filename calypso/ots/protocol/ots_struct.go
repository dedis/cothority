package protocol

import (
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

const NameOTS = "OTS"

func init() {
	network.RegisterMessages(&Reencrypt{}, &ReencryptReply{})
}

type VerifyRequest func(rc *Reencrypt, idx int) (*pvss.PubVerShare,
	kyber.Point, darc.ID)

type Reencrypt struct {
	Xc               kyber.Point
	VerificationData *[]byte
}

type structReencrypt struct {
	*onet.TreeNode
	Reencrypt
}

type ReencryptReply struct {
	Index int
	Egp   *EGP
}

type structReencryptReply struct {
	*onet.TreeNode
	ReencryptReply
}

type EGP struct {
	K  kyber.Point
	Cs []kyber.Point
}

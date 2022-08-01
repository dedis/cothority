package protocol

import (
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

const NamePQOTS = "PQOTS"

func init() {
	network.RegisterMessages(&Reencrypt{}, &ReencryptReply{})
}

type VerifyRequest func(rc *Reencrypt) bool

type GetShare func([]byte) (*share.PriShare, error)

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

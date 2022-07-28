package pq

import (
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
)

type Write struct {
	CtxtHash    []byte
	Publics     []kyber.Point
	Commitments [][]byte
	//TODO: DARC policy
}

type VerifyWrite struct {
	Idx   int
	Write *Write
	Share *share.PriShare
	Rand  []byte
}

type VerifyWriteReply struct {
	Sig []byte
}

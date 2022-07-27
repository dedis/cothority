package pq

import (
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	network.RegisterMessages(VerifyWrite{}, VerifyWriteReply{})
}

type Write struct {
	Commitments [][]byte
	Publics     []kyber.Point
	CtxtHash    []byte
	//TODO: DARC policy
}

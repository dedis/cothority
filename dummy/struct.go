package dummy

import (
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
)

type dummyData struct {
	DKID    string
	XhatEnc kyber.Point
}

type DummyRequest struct {
	Roster *onet.Roster
	DKID   string
}

type DummyReply struct {
	Signature protocol.BlsSignature
}

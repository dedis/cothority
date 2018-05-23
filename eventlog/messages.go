package eventlog

import (
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

func init() {
	network.RegisterMessages(&InitRequest{},
		&InitResponse{},
		&LogRequest{},
		&LogResponse{},
	)
}

type InitRequest struct {
	Writer darc.Darc
	Roster onet.Roster
}

type InitResponse struct {
	ID skipchain.SkipBlockID
}

type LogRequest struct {
	Topic string
	Event string
}

type LogResponse struct {
}

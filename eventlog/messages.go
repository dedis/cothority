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

// InitRequest is sent to start a new EventLog.
type InitRequest struct {
	Writer darc.Darc
	Roster onet.Roster
}

// InitResponse is the reply to InitRequest.
type InitResponse struct {
	ID skipchain.SkipBlockID
}

// LogRequest is sent to create an event log.
type LogRequest struct {
	Topic string
	Event string
}

// LogResponse is the reply to LogRequest.
type LogResponse struct {
}

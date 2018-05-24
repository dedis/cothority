package eventlog

import (
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

func init() {
	network.RegisterMessages(&InitRequest{},
		&InitResponse{},
		&Event{},
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

// LogRequest is a wrapper around omniledger.AddTxRequest for use in the
// event logger.
type LogRequest struct {
	omniledger.AddTxRequest
}

// Event is sent to create an event log.
type Event struct {
	Topic   string
	Content string
}

// LogResponse is the reply to Content.
type LogResponse struct {
}

package eventlog

import (
	"time"

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
		&GetEventRequest{},
		&GetEventResponse{},
	)
}

// PROTOSTART
// import "roster.proto";
// import "darc.proto";
// import "transaction.proto";
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "EventLogProto";

// ***
// These are the messages used in the API-calls
// ***

// InitRequest is sent to start a new EventLog.
type InitRequest struct {
	Owner  darc.Darc
	Roster onet.Roster
}

// InitResponse is the reply to InitRequest.
type InitResponse struct {
	ID skipchain.SkipBlockID
}

// LogRequest is puts one or more new log events into the OmniLedger.
type LogRequest struct {
	SkipchainID skipchain.SkipBlockID
	Transaction omniledger.ClientTransaction
}

// Event is sent to create an event log.
type Event struct {
	When    int64
	Topic   string
	Content string
}

// LogResponse is the reply to LogRequest.
type LogResponse struct {
}

// GetEventRequest is sent to get an event.
type GetEventRequest struct {
	SkipchainID skipchain.SkipBlockID
	Key         []byte
}

// GetEventResponse is the reply of GetEventRequest.
type GetEventResponse struct {
	Event Event
}

// NewEvent returns a new event with the current time sec correctly.
func NewEvent(topic, content string) Event {
	return Event{
		When:    time.Now().UnixNano(),
		Topic:   topic,
		Content: content,
	}
}

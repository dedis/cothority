package eventlog

import (
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

func init() {
	network.RegisterMessages(&InitRequest{}, &InitResponse{},
		&Event{},
		&LogRequest{}, &LogResponse{},
		&GetEventRequest{}, &GetEventResponse{},
		&SearchRequest{}, &SearchResponse{},
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
	Owner         darc.Darc
	Roster        onet.Roster
	BlockInterval time.Duration
}

// InitResponse is the reply to InitRequest.
type InitResponse struct {
	ID skipchain.SkipBlockID
}

// LogRequest puts one or more new log events into the OmniLedger.
type LogRequest struct {
	SkipchainID skipchain.SkipBlockID
	Transaction omniledger.ClientTransaction
}

// LogResponse is the reply to LogRequest.
type LogResponse struct {
}

// SearchRequest includes all the search parameters (AND of all provided search parameters).
// Topic == "" means "any topic". From == 0 means "from the first event", and To == 0
// means "until now". From and To should be set using the UnixNano() method in package time.
type SearchRequest struct {
	ID    skipchain.SkipBlockID
	Topic string // Return events where Event.Topic == Topic, if Topic != "".
	From  int64  // Return events where When is > From.
	To    int64  // Return events where When is <= To.
}

// SearchResponse is the reply to LogRequest.
type SearchResponse struct {
	Events []Event
	// Events does not contain all the results. The caller should formulate
	// a new SearchRequest to continue searching, for instance by setting
	// From to the time of the last received event.
	Truncated bool
}

// Event is sent to create an event log. When should be set using the UnixNano() method
// in package time.
type Event struct {
	When    int64
	Topic   string
	Content string
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

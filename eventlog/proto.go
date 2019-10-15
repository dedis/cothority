package eventlog

import (
	"time"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v4/network"
)

func init() {
	network.RegisterMessages(
		&Event{},
		&SearchRequest{}, &SearchResponse{},
	)
}

// NewEvent returns a new event mapping with the current time as its
// timestamp and a random key.
func NewEvent(topic, content string) Event {
	return Event{
		When:    time.Now().UnixNano(),
		Topic:   topic,
		Content: content,
	}
}

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
// type :byzcoin.InstanceID:bytes
//
// package eventlog;
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "EventLogProto";

// ***
// These are the messages used in the API-calls
// ***

// SearchRequest includes all the search parameters (AND of all provided search
// parameters). Topic == "" means "any topic". From == 0 means "from the first
// event", and To == 0 means "until now". From and To should be set using the
// UnixNano() method in package time.
type SearchRequest struct {
	Instance byzcoin.InstanceID
	ID       skipchain.SkipBlockID
	// Return events where Event.Topic == Topic, if Topic != "".
	Topic string
	// Return events where When is > From.
	From int64
	// Return events where When is <= To.
	To int64
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

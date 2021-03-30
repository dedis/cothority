package bypros

import (
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/network"
)

// Follow is a request to start following the chain.
type Follow struct {
	// ScID is the skipchain ID of the chain we want to follow
	ScID skipchain.SkipBlockID

	// Target is the conode to be followed
	Target *network.ServerIdentity
}

// EmptyReply is an empty reply.
type EmptyReply struct {
}

// Unfollow is a request to stop following.
type Unfollow struct{}

// Query is a request to send an SQL query.
type Query struct {
	// Query is an SQL query. Note that only read-only query will be accepted.
	Query string
}

// QueryReply is a response to a Query request.
type QueryReply struct {
	// Result is a json marshalled result of the query.
	Result []byte
}

// CatchUpMsg is a request to initiate a catch-up. All blocks not already
// present in the db will be stored.
type CatchUpMsg struct {
	// ScID is the skipchain ID of the chain we want to catch up
	ScID skipchain.SkipBlockID

	// Target is the conode from which to perform the catch up.
	Target *network.ServerIdentity

	// FromBlock is the first block from where to start the catch up. The chain
	// will be browsed from this block until the end of the chain.
	FromBlock skipchain.SkipBlockID

	// UpdateEvery specifies the interval at which a response should be sent
	// back to the client. A value of 2 means we will get an update every 2
	// blocks parsed.
	UpdateEvery int
}

// CatchUpResponse is the response sent back to a catch up request. This
// response is sent periodically.
type CatchUpResponse struct {
	// Status a status message about the progress
	Status CatchUpStatus

	// Done is set to true when there is no blocks left to parse.
	Done bool

	// Err is filled when something unexpected happened and the process is
	// aborted.
	Err string
}

// CatchUpStatus describes a status message sent back to the client when a catch
// up is initiated.
type CatchUpStatus struct {
	Message    string
	BlockIndex int
	BlockHash  []byte
}

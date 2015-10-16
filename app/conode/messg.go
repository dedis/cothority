package main

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/app/conode/defs"
)

// struct to ease keeping track of who requires a reply after
// tsm is processed/ aggregated by the TSServer
type MustReplyMessage struct {
	Tsm defs.TimeStampMessage
	To  string // name of reply destination
}

type LogEntry struct {
	Seq  defs.SeqNo         // Consecutively-incrementing log entry sequence number
	Root hashid.HashId // Merkle root of values committed this time-step
	Time *int64        // Optional wall-clock time this entry was created
}

type SignedEntry struct {
	Ent []byte // Encoded LogEntry to which the signature applies
	Sig []byte // Digital signature on the LogEntry
}

// Request to obtain an old log-entry and, optionally,
// a cryptographic proof that it happened before a given newer entry.
// The TSServer may be unable to process if Seq is beyond the retention window.
type EntryRequest struct {
	Seq defs.SeqNo // Sequence number of old entry requested
}
type EntryReply struct {
	Log SignedEntry // Signed log entry
}

// Request a cryptographic Merkle proof that log-entry Old happened before New.
// Produces a path to a Merkle tree node containing a hash of the node itself
// and the root of the history values committed within the node.
// The TSServer may be unable to process if Old is beyond the retention window.
type ProofRequest struct {
	Old, New defs.SeqNo // Sequence number of old and new log records
}
type ProofReply struct {
	Prf proof.Proof // Requested Merkle proof
}

// XXX not sure we need block requests?
type BlockRequest struct {
	Ids []hashid.HashId // Hash of block(s) requested
}

type BlockReply struct {
	Dat [][]byte // Content of block(s) requested
}

type ErrorReply struct {
	Msg string // Human-readable error message
}

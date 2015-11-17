package main

import (
	"bytes"
	"encoding/gob"

	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
)

type SeqNo byte

// struct to ease keeping track of who requires a reply after
// tsm is processed/ aggregated by the TSServer
type MustReplyMessage struct {
	Tsm TimeStampMessage
	To  string // name of reply destination
}

type LogEntry struct {
	Seq  SeqNo         // Consecutively-incrementing log entry sequence number
	Root hashid.HashId // Merkle root of values committed this time-step
	Time *int64        // Optional wall-clock time this entry was created
}

type SignedEntry struct {
	Ent []byte // Encoded LogEntry to which the signature applies
	Sig []byte // Digital signature on the LogEntry
}

type StampRequest struct {
	Val []byte // Hash-size value to timestamp
}
type StampSignature struct {
	Sig []byte      // Signature on the root
	Prf proof.Proof // Merkle proof of value
}

// Request to obtain an old log-entry and, optionally,
// a cryptographic proof that it happened before a given newer entry.
// The TSServer may be unable to process if Seq is beyond the retention window.
type EntryRequest struct {
	Seq SeqNo // Sequence number of old entry requested
}
type EntryReply struct {
	Log SignedEntry // Signed log entry
}

// Request a cryptographic Merkle proof that log-entry Old happened before New.
// Produces a path to a Merkle tree node containing a hash of the node itself
// and the root of the history values committed within the node.
// The TSServer may be unable to process if Old is beyond the retention window.
type ProofRequest struct {
	Old, New SeqNo // Sequence number of old and new log records
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

type MessageType int

const (
	Error MessageType = iota
	StampRequestType
	StampSignatureType
)

type TimeStampMessage struct {
	ReqNo SeqNo // Request sequence number
	// ErrorReply *ErrorReply // Generic error reply to any request
	Type MessageType
	Sreq *StampRequest
	Srep *StampSignature
}

func (tsm TimeStampMessage) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	var sub []byte
	var err error
	b.WriteByte(byte(tsm.Type))
	b.WriteByte(byte(tsm.ReqNo))
	// marshal sub message based on its Type
	switch tsm.Type {
	case StampRequestType:
		sub, err = tsm.Sreq.MarshalBinary()
	case StampSignatureType:
		sub, err = tsm.Srep.MarshalBinary()
	}
	if err == nil {
		b.Write(sub)
	}
	return b.Bytes(), err
}

func (sm *TimeStampMessage) UnmarshalBinary(data []byte) error {
	sm.Type = MessageType(data[0])
	sm.ReqNo = SeqNo(data[1])
	msgBytes := data[2:]
	var err error
	switch sm.Type {
	case StampRequestType:
		sm.Sreq = &StampRequest{}
		err = sm.Sreq.UnmarshalBinary(msgBytes)
	case StampSignatureType:
		sm.Srep = &StampSignature{}
		err = sm.Srep.UnmarshalBinary(msgBytes)

	}
	return err
}

func (Sreq StampRequest) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(Sreq.Val)
	return b.Bytes(), err
}

func (Sreq *StampRequest) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	dec := gob.NewDecoder(b)
	err := dec.Decode(&Sreq.Val)
	return err
}

func (Srep StampSignature) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(Srep.Sig)
	return b.Bytes(), err
}

func (Srep *StampSignature) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	dec := gob.NewDecoder(b)
	err := dec.Decode(&Srep.Sig)
	return err
}

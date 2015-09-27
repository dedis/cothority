package main

import (
	"github.com/dedis/crypto/abstract"
	"time"
)

type HostInfo struct {
	PubKey   []byte  // Fingerprint of host's public key
	Location *string // Optional network location hint, hostname[:port]
}

// Session information uniquely identifying a RandHound protocol run.
type Session struct {
	Client  HostInfo  // Initiating client
	Purpose string    // Purpose of randomness
	Time    time.Time // Scheduled initiation time
}

// Group parameters for a protocol run
type Group struct {
	Servers []HostInfo // List of servers used
	F       int        // Faulty (Byzantine) hosts tolerated
	L       int        // Hosts that must be live
	K       int        // Trustee set size
	T       int        // Trustee set threshold
}

type I1 struct {
	SID []byte // Session identifier: hash of Session info block
	GID []byte // Group identifier: hash of Group parameter block
	HRc []byte // Client's trustee-randomness commit
	S   []byte // Optional: full session info block
	G   []byte // Optional: full group parameter block
}

type R1 struct {
	HI1 []byte // Hash of I1 message responding to
	HRs []byte // Server's trustee-randomness commit
}

type I2 struct {
	SID []byte // Unique session identifier
	Rc  []byte // Client's trustee-selection randomness
}

type R2 struct {
	HI2  []byte // Hash of I2 message responding to
	Rs   []byte // Server's trustee-selection randomness
	Deal []byte // Server's secret-sharing to trustees
}

type I3 struct {
	SID []byte // Unique session identifier

	// Client's list of signed R2 messages;
	// empty slices represent missing R2 messages.
	R2s [][]byte
}

type R3 struct {
	HI3  []byte   // Hash of I3 message responding to
	Resp []R3Resp // Responses to dealt secret-shares
}

type R3Resp struct {
	Dealer int    // Server number of dealer
	Index  int    // Share number in deal we are validating
	Resp   []byte // Encoded response to dealer's Deal
}

type I4 struct {
	SID []byte // Unique session identifier
	// Client's list of signed R2 messages;
	// empty slices for missing or invalid R2s.
	R2s [][]byte
}

type R4 struct {
	HI4    []byte    // Hash of I4 message responding to
	Shares []R4Share // Revealed secret-shares
}

type R4Share struct {
	Dealer int             // Server number of dealer
	Index  int             // Share number in dealer's Deal
	Share  abstract.Secret // Decrypted share dealt to this server
}

// Error response reported by a server
type RError struct {
	Code   ErrorCode // Machine-readable error code
	String string    // Human-readable error message
}

type ErrorCode int

const (
	NoError ErrorCode = iota
	UnknownSessionID
	UnknownGroupID
)

// IMessage represents any message a RandHound initiator/client can send.
// The fields of this struct should be treated as a protobuf 'oneof'.
type IMessage struct {
	I1 *I1
	I2 *I2
	I3 *I3
	I4 *I4
}

// RMessage represents any message a RandHound responder/server can send.
// The fields of this struct should be treated as a protobuf 'oneof'.
type RMessage struct {
	RE *RError
	R1 *R1
	R2 *R2
	R3 *R3
	R4 *R4
}

type Transcript struct {
	I1 []byte   // I1 message signed by client
	R1 [][]byte // R1 messages signed by resp servers
	I2 []byte
	R2 [][]byte
	I3 []byte
	R3 [][]byte
	I4 []byte
	R4 [][]byte
}

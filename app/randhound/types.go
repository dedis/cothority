package randhound

import (
	"time"

	"github.com/dedis/crypto/abstract"
)

// TODO: figure out which of the old RandHound types (see app/rand/types.go)
// are necessary and which ones are covered by SDA

type Session struct {
	pubk    abstract.Point // Public key of the root node
	Purpose string         // Purpose of randomness
	Time    time.Time      // Scheduled initiation time
}

type Group struct {
}

type I1 struct {
	SID []byte // Session identifier: hash of session info block
	GID []byte // Group identifier: hash of group parameter block
	HRc []byte // Client's trustee-randomness commit
	S   []byte // Full session info block (optional)
	G   []byte // Full group parameter block (optional)
}

type R1 struct {
	HI1 []byte // Hash of I1 message
	HRs []byte // Server's trustee-randomness commit
}

type I2 struct {
	SID []byte // Session identifier
	Rc  []byte // Client's trustee-selection randomness
}

type R2 struct {
	HI2  []byte // Hash of I2 message
	Rs   []byte // Servers' trustee-selection randomness
	Deal []byte // Server's secret-sharing to trustees
}

type I3 struct {
	SID []byte   // Session identifier
	R2s [][]byte // Client's list of signed R2 messages; empty slices represent missing R2 messages
}

type R3 struct {
	HI3  []byte   // Hash of I3 message
	Resp []R3Resp // Responses to dealt secret-shares
}

type R3Resp struct {
	Dealer int    // Server number of dealer
	Index  int    // Share number in deal we are validating
	Resp   []byte // Encoded response to dealer's Deal
}

type I4 struct {
	SID []byte   // Session identifier
	R2s [][]byte // Client's list of signed R2 messages; empty slices represent missing R2 messages
}

type R4 struct {
	HI4    []byte    // Hash of I4 message
	Shares []R4Share // Revealed secret-shares
}

type R4Share struct {
	Dealer int             // Server number of dealer
	Index  int             // Share number in dealer's Deal
	Share  abstract.Secret // Decrypted share dealt to this server
}

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

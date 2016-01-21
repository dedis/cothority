package randhound

import (
	"time"

	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

var Done chan bool
var Purpose chan string
var Trustees chan int

var TypeI1 uuid.UUID
var TypeR1 uuid.UUID
var TypeI2 uuid.UUID
var TypeR2 uuid.UUID
var TypeI3 uuid.UUID
var TypeR3 uuid.UUID
var TypeI4 uuid.UUID
var TypeR4 uuid.UUID

// TODO: figure out which of the old RandHound types (see app/rand/types.go)
// are necessary and which ones are covered by SDA

type Session struct {
	LPubKey []byte    // Finger print of leader's public key
	Purpose string    // Purpose of randomness
	Time    time.Time // Scheduled initiation time
}

type Group struct {
	PPubKey [][]byte // Finger prints of peers' public keys
	F       uint32   // Faulty (Byzantine) hosts tolerated
	L       uint32   // Hosts that must be live
	K       uint32   // Trustee set size
	T       uint32   // Trustee set threshold
}

type I1 struct {
	SID []byte // Session identifier: hash of session info block
	GID []byte // Group identifier: hash of group parameter block
	HRc []byte // Client's trustee-randomness commit
	//S   []byte // Full session info block (optional)
	//G   []byte // Full group parameter block (optional)
}

type R1 struct {
	HI1 []byte // Hash of I1 message
	HRs []byte // Peer's trustee-randomness commit
}

type I2 struct {
	SID []byte // Session identifier
	Rc  []byte // Leader's trustee-selection randomness
}

type R2 struct {
	HI2    []byte // Hash of I2 message
	Rs     []byte // Peers' trustee-selection randomness
	Dealer int    // Dealer's index in the peer list
	Deal   []byte // Peer's secret-sharing to trustees
}

type I3 struct {
	SID []byte // Session identifier
	R2s []R2   // Client's list of signed R2 messages; empty slices represent missing R2 messages
}

type R3 struct {
	HI3  []byte   // Hash of I3 message
	Resp []R3Resp // Responses to dealt secret-shares
}

type R3Resp struct {
	Dealer int    // Dealer's index in the peer list
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
	Dealer int             // Dealer's index in the peer list
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
	//RE *RError
	R1 *R1
	R2 *R2
	R3 *R3
	R4 *R4
}

type Transcript struct {
	I1 []byte   // I1 message signed by leader
	R1 [][]byte // R1 messages signed by resp peers
	I2 []byte   // I2 message signed by leader
	R2 [][]byte // R2 messages signed by resp peers
	I3 []byte   // I3 message signed by leader
	R3 [][]byte // R3 messages signed by resp peers
	I4 []byte   // I4 message signed by leader
	R4 [][]byte // R4 messages signed by resp peers
}

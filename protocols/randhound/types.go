package randhound

import (
	"time"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

type Session struct {
	Fingerprint []byte    // Fingerprint of a public key (usually of the leader)
	Purpose     string    // Purpose of randomness
	Time        time.Time // Scheduled initiation time
}

type Group struct {
	N int // Total number of nodes (peers + leader)
	F int // Maximum number of Byzantine nodes tolerated (1/3)
	L int // Minimum number of non-Byzantine nodes required (2/3)
	K int // Total number of trustees (= shares generated per peer)
	R int // Minimum number of signatures needed to certify a deal
	T int // Minimum number of shares needed to reconstruct a secret
}

type I1 struct {
	SID     []byte   // Session identifier: hash of session info block
	Session *Session // Session parameters
	GID     []byte   // Group identifier: hash of group parameter block
	Group   *Group   // Group parameters
	HRc     []byte   // Client's trustee-randomness commit
}

type R1 struct {
	Src int    // Source of the message
	HI1 []byte // Hash of I1 message
	HRs []byte // Peer's trustee-randomness commit
}

type I2 struct {
	SID []byte // Session identifier
	Rc  []byte // Leader's trustee-selection randomness
}

type R2 struct {
	Src  int    // Source of the message
	HI2  []byte // Hash of I2 message
	Rs   []byte // Peers' trustee-selection randomness
	Deal []byte // Peer's secret-sharing to trustees
}

type I3 struct {
	SID []byte      // Session identifier
	R2s map[int]*R2 // Leaders's list of signed R2 messages; empty slices represent missing R2 messages
}

type R3 struct {
	Src       int      // Source of the message
	HI3       []byte   // Hash of I3 message
	Responses []R3Resp // Responses to dealt secret-shares
}

type R3Resp struct {
	Dealer   int    // Dealer's index in the peer list
	ShareIdx int    // Share index in deal we are validating
	Resp     []byte // Encoded response to dealer's deal
}

// TODO: instead of re-transmitting the full vector of R2 messages, just form a
// bit-vector that indicates which of the previously transmitted R2 messages are
// good/bad
type I4 struct {
	SID []byte      // Session identifier
	R2s map[int]*R2 // Leader's list of signed R2 messages; empty slices represent missing R2 messages
}

type R4 struct {
	Src    int       // Source of the message
	HI4    []byte    // Hash of I4 message
	Shares []R4Share // Revealed secret-shares
}

type R4Share struct {
	Dealer int             // Dealer's index in the peer list
	Idx    int             // Share index in dealer's deal
	Share  abstract.Secret // Decrypted share dealt to this server
}

// SDA-wrapper around I1
type WI1 struct {
	*sda.TreeNode
	I1
}

// SDA-wrapper around I2
type WI2 struct {
	*sda.TreeNode
	I2
}

// SDA-wrapper around I3
type WI3 struct {
	*sda.TreeNode
	I3
}

// SDA-wrapper around I4
type WI4 struct {
	*sda.TreeNode
	I4
}

// SDA-wrapper around R1
type WR1 struct {
	*sda.TreeNode
	R1
}

// SDA-wrapper around R2
type WR2 struct {
	*sda.TreeNode
	R2
}

// SDA-wrapper around R3
type WR3 struct {
	*sda.TreeNode
	R3
}

// SDA-wrapper around R4
type WR4 struct {
	*sda.TreeNode
	R4
}

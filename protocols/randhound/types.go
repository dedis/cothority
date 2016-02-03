package randhound

import (
	"time"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

//var Done chan bool
//var Result chan []byte

type Session struct {
	LPubKey []byte    // Finger print of leader's public key
	Purpose string    // Purpose of randomness
	Time    time.Time // Scheduled initiation time
}

type Group struct {
	PPubKey [][]byte // Finger prints of peers' public keys
	F       int      // Faulty (Byzantine) hosts tolerated (1/3)
	L       int      // Hosts that must be live (2/3)
	K       int      // Trustee set size
	T       int      // Trustee set threshold
}

type I1 struct {
	SID []byte // Session identifier: hash of session info block
	GID []byte // Group identifier: hash of group parameter block
	HRc []byte // Client's trustee-randomness commit
	//S   []byte // Full session info block (optional)
	//G   []byte // Full group parameter block (optional)
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
	SID []byte // Session identifier
	R2s []R2   // Leaders's list of signed R2 messages; empty slices represent missing R2 messages
}

type R3 struct {
	Src  int      // Source of the message
	HI3  []byte   // Hash of I3 message
	Resp []R3Resp // Responses to dealt secret-shares
}

type R3Resp struct {
	Dealer int    // Dealer's index in the peer list
	Index  int    // Share number in deal we are validating
	Resp   []byte // Encoded response to dealer's deal
}

// TODO: instead of re-transmitting the full vector of R2 messages, just form a
// bit-vector that indicates which of the previously tranmitted R2 messages are
// good/bad
type I4 struct {
	SID []byte // Session identifier
	R2s []R2   // Leader's list of signed R2 messages; empty slices represent missing R2 messages
}

type R4 struct {
	Src    int       // Source of the message
	HI4    []byte    // Hash of I4 message
	Shares []R4Share // Revealed secret-shares
}

type R4Share struct {
	Dealer int             // Dealer's index in the peer list
	Index  int             // Share number in dealer's deal
	Share  abstract.Secret // Decrypted share dealt to this server
}

type StructI1 struct {
	*sda.TreeNode
	I1
}

type StructI2 struct {
	*sda.TreeNode
	I2
}

type StructI3 struct {
	*sda.TreeNode
	I3
}

type StructI4 struct {
	*sda.TreeNode
	I4
}

type StructR1 struct {
	*sda.TreeNode
	R1
}

type StructR2 struct {
	*sda.TreeNode
	R2
}

type StructR3 struct {
	*sda.TreeNode
	R3
}

type StructR4 struct {
	*sda.TreeNode
	R4
}

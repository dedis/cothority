// Collection of some types used in the RandHound protocol.
package randhound

import (
	"time"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// Session encapsulates some metadata on a RandHound protocol run.
type Session struct {
	Fingerprint []byte    // Fingerprint of a public key (usually of the leader)
	Purpose     string    // Purpose of randomness
	Time        time.Time // Scheduled initiation time
}

// Group encapsulates all the configuration parameters of a list of RandHound nodes.
type Group struct {
	N uint32 // Total number of nodes (peers + leader)
	F uint32 // Maximum number of Byzantine nodes tolerated (1/3)
	L uint32 // Minimum number of non-Byzantine nodes required (2/3)
	K uint32 // Total number of trustees (= shares generated per peer)
	R uint32 // Minimum number of signatures needed to certify a deal
	T uint32 // Minimum number of shares needed to reconstruct a secret
}

// I1 is the message sent from the leader to all the peers in phase 1.
type I1 struct {
	SID     []byte   // Session identifier: hash of session info block
	Session *Session // Session parameters
	GID     []byte   // Group identifier: hash of group parameter block
	Group   *Group   // Group parameters
	HRc     []byte   // Client's trustee-randomness commit
}

// R1 is the reply sent from all the peers to the leader in phase 1.
type R1 struct {
	Src uint32 // Source of the message
	HI1 []byte // Hash of I1 message
	HRs []byte // Peer's trustee-randomness commit
}

// I2 is the message sent from the leader to all the peers in phase 2.
type I2 struct {
	SID []byte // Session identifier
	Rc  []byte // Leader's trustee-selection randomnes
}

// R2 is the reply sent from all the peers to the leader in phase 2.
type R2 struct {
	Src  uint32 // Source of the message
	HI2  []byte // Hash of I2 message
	Rs   []byte // Peers' trustee-selection randomness
	Deal []byte // Peer's secret-sharing to trustees
}

// I3 is the message sent from the leader to all the peers in phase 3.
type I3 struct {
	SID []byte         // Session identifier
	R2s map[uint32]*R2 // Leaders's list of signed R2 messages; empty slices represent missing R2 messages
}

// R3 is the reply sent from all the peers to the leader in phase 3.
type R3 struct {
	Src       uint32   // Source of the message
	HI3       []byte   // Hash of I3 message
	Responses []R3Resp // Responses to dealt secret-shares
}

// R3Resp encapsulates a peer's reply together with some metadata.
type R3Resp struct {
	DealerIdx uint32 // Dealer's index in the peer list
	ShareIdx  uint32 // Share's index in deal we are validating
	Resp      []byte // Encoded response to dealer's deal
}

// I4 is the message sent from the leader to all the peers in phase 4.
type I4 struct {
	SID     []byte               // Session identifier
	Invalid map[uint32]*[]uint32 // Map to mark invalid responses
}

// R4 is the reply sent from all the peers to the leader in phase 4.
type R4 struct {
	Src    uint32              // Source of the message
	HI4    []byte              // Hash of I4 message
	Shares map[uint32]*R4Share // Revealed secret-shares
}

// R4Share encapsulates a peer's share together with some metadata.
type R4Share struct {
	DealerIdx uint32          // Dealer's index in the peer list
	ShareIdx  uint32          // Share's index in dealer's deal
	Share     abstract.Secret // Decrypted share dealt to this server
}

// WI1 is a SDA-wrapper around I1
type WI1 struct {
	*sda.TreeNode
	I1
}

// WI2 is a SDA-wrapper around I2
type WI2 struct {
	*sda.TreeNode
	I2
}

// WI3 is a SDA-wrapper around I3
type WI3 struct {
	*sda.TreeNode
	I3
}

// WI4 is a SDA-wrapper around I4
type WI4 struct {
	*sda.TreeNode
	I4
}

// WR1 is a SDA-wrapper around R1
type WR1 struct {
	*sda.TreeNode
	R1
}

// WR2 is a SDA-wrapper around R2
type WR2 struct {
	*sda.TreeNode
	R2
}

// WR3 is a SDA-wrapper around R3
type WR3 struct {
	*sda.TreeNode
	R3
}

// WR4 is a SDA-wrapper around R4
type WR4 struct {
	*sda.TreeNode
	R4
}

package randhound

import (
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// RandHound ...
type RandHound struct {
	*sda.TreeNodeInstance

	// Session information
	Nodes   int       // Total number of nodes (client + servers)
	Groups  int       // Number of groups
	Faulty  int       // Maximum number of Byzantine servers
	Purpose string    // Purpose of the protocol run
	Time    time.Time // Timestamp of initiation
	CliRand []byte    // Client-chosen randomness
	SID     []byte    // Session identifier

	// Group information
	Server              [][]*sda.TreeNode  // Grouped servers
	Threshold           []int              // Group thresholds
	Key                 [][]abstract.Point // Grouped server public keys
	ServerIdxToGroupNum []int              // Mapping of gloabl server index to group number
	ServerIdxToGroupIdx []int              // Mapping of global server index to group server index

	// Message information
	HashI1 map[int][]byte           // Hash of I1 message (index: group)
	HashI2 map[int][]byte           // Hash of I2 message (index: server)
	R1s    map[int]*R1              // R1 messages received from servers
	R2s    map[int]*R2              // R2 messages received from servers
	CR1    []int                    // Number of received R1 messages per group
	CR2    []int                    // Number of received R2 messages per group
	Commit map[int][]abstract.Point // Commitments of server polynomials (index: server)
	mutex  sync.Mutex

	// For signaling the end of a protocol run
	Done chan bool

	// XXX: Dummy, remove later
	counter int
}

// Transcript ...
type Transcript struct {
	SID       []byte                 // Session identifier
	Nodes     int                    // Total number of nodes (client + server)
	Faulty    int                    // Maximum number of Byzantine servers
	Purpose   string                 // Purpose of the protocol run
	Time      time.Time              // Timestamp of initiation
	CliRand   []byte                 // Client-chosen randomness (for sharding)
	Threshold []int                  // Grouped secret sharing thresholds
	Key       [][]abstract.Point     // Grouped public keys
	R1s       [][]*R1                // Grouped R1 messages received from servers
	R2s       [][]*R2                // Grouped R2 messages received from servers
	SigI1     []*crypto.SchnorrSig   // Grouped Schnorr signatures of I1 messages
	SigI2     [][]*crypto.SchnorrSig // Grouped Schnorr signatures of I2 messages
	SigR1     [][]*crypto.SchnorrSig // Grouped Schnorr signatures of R1 messages
	SigR2     [][]*crypto.SchnorrSig // Grouped Schnorr signatures of R2 messages
}

// I1 message
type I1 struct {
	Sig       crypto.SchnorrSig // Schnorr signature
	SID       []byte            // Session identifier
	Threshold int               // Secret sharing threshold
	Key       []abstract.Point  // Public keys of trustees
}

// R1 message
type R1 struct {
	Sig        crypto.SchnorrSig // Schnorr signature
	HI1        []byte            // Hash of I1
	EncShare   []abstract.Point  // Encrypted shares
	EncProof   []ProofCore       // Encryption consistency proofs
	CommitPoly []byte            // Marshalled commitment polynomial
}

// I2 message
type I2 struct {
	Sig      crypto.SchnorrSig // Schnorr signature
	SID      []byte            // Session identifier
	EncShare []abstract.Point  // Encrypted shares
	EncProof []ProofCore       // Encryption consistency proofs
	Commit   []abstract.Point  // Polynomial commitments
}

// R2 message
type R2 struct {
	Sig      crypto.SchnorrSig // Schnorr signature
	HI2      []byte            // Hash of I2
	DecShare []abstract.Point  // Decrypted shares
	DecProof []ProofCore       // Decryption consistency proofs
}

// WI1 is a SDA-wrapper around I1
type WI1 struct {
	*sda.TreeNode
	I1
}

// WR1 is a SDA-wrapper around R1
type WR1 struct {
	*sda.TreeNode
	R1
}

// WI2 is a SDA-wrapper around I2
type WI2 struct {
	*sda.TreeNode
	I2
}

// WR2 is a SDA-wrapper around R2
type WR2 struct {
	*sda.TreeNode
	R2
}

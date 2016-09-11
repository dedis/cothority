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
	Group               [][]int            // Grouped server indices
	Threshold           []int              // Group thresholds
	Key                 [][]abstract.Point // Grouped server public keys
	ServerIdxToGroupNum []int              // Mapping of gloabl server index to group number
	ServerIdxToGroupIdx []int              // Mapping of global server index to group server index

	// Message information
	I1s          map[int]*I1              // I1 messages sent to servers (index: group)
	I2s          map[int]*I2              // I2 messages sent to servers (index: server)
	R1s          map[int]*R1              // R1 messages received from servers (index: server)
	R2s          map[int]*R2              // R2 messages received from servers (index: server)
	PolyCommit   map[int][]abstract.Point // Commitments of server polynomials (index: server)
	Secret       map[int][]int            // Valid shares per secret/server (source server index -> list of target server indices)
	ChosenSecret map[int][]int            // Chosen secrets that contribute to collective randomness

	mutex sync.Mutex

	// For signaling the end of a protocol run
	Done        chan bool
	SecretReady bool
	Counter     int
}

// Share ...
type Share struct {
	Source int            // Source server index
	Target int            // Target server index
	Gen    int            // Share generation index
	Val    abstract.Point // Share value
}

// Transcript ...
type Transcript struct {
	SID          []byte             // Session identifier
	Nodes        int                // Total number of nodes (client + server)
	Groups       int                // Number of groups
	Faulty       int                // Maximum number of Byzantine servers
	Purpose      string             // Purpose of the protocol run
	Time         time.Time          // Timestamp of initiation
	CliRand      []byte             // Client-chosen randomness (for sharding)
	CliKey       abstract.Point     // Client public key
	Group        [][]int            // Grouped server indices
	Key          [][]abstract.Point // Grouped server public keys
	Threshold    []int              // Grouped secret sharing thresholds
	ChosenSecret map[int][]int      // Chosen secrets that contribute to collective randomness
	I1s          map[int]*I1        // I1 messages sent to servers
	I2s          map[int]*I2        // I2 messages sent to servers
	R1s          map[int]*R1        // R1 messages received from servers
	R2s          map[int]*R2        // R2 messages received from servers
}

// I1 message
type I1 struct {
	Sig       crypto.SchnorrSig // Schnorr signature
	SID       []byte            // Session identifier
	Threshold int               // Secret sharing threshold
	Group     []uint32          // Group indices
	Key       []abstract.Point  // Public keys of trustees
}

// R1 message
type R1 struct {
	Sig        crypto.SchnorrSig // Schnorr signature
	HI1        []byte            // Hash of I1
	EncShare   []Share           // Encrypted shares
	EncProof   []ProofCore       // Encryption consistency proofs
	CommitPoly []byte            // Marshalled commitment polynomial
}

// I2 message
type I2 struct {
	Sig          crypto.SchnorrSig // Schnorr signature
	SID          []byte            // Session identifier
	ChosenSecret [][]uint32        // Chosen secrets
	EncShare     []Share           // Encrypted shares
	EncProof     []ProofCore       // Encryption consistency proofs
	PolyCommit   []abstract.Point  // Polynomial commitments
}

// R2 message
type R2 struct {
	Sig      crypto.SchnorrSig // Schnorr signature
	HI2      []byte            // Hash of I2
	DecShare []Share           // Decrypted shares
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

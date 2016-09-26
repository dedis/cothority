package randhound

import (
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// RandHound is the main protocol struct and implements the
// sda.ProtocolInstance interface.
type RandHound struct {
	*sda.TreeNodeInstance

	mutex sync.Mutex

	// Session information
	nodes   int       // Total number of nodes (client + servers)
	groups  int       // Number of groups
	faulty  int       // Maximum number of Byzantine servers
	purpose string    // Purpose of protocol run
	time    time.Time // Timestamp of initiation
	cliRand []byte    // Client-chosen randomness (for initial sharding)
	sid     []byte    // Session identifier

	// Group information
	server              [][]*sda.TreeNode  // Grouped servers
	group               [][]int            // Grouped server indices
	threshold           []int              // Group thresholds
	key                 [][]abstract.Point // Grouped server public keys
	ServerIdxToGroupNum []int              // Mapping of gloabl server index to group number
	ServerIdxToGroupIdx []int              // Mapping of global server index to group server index

	// Message information
	i1s          map[int]*I1              // I1 messages sent to servers (index: group)
	i2s          map[int]*I2              // I2 messages sent to servers (index: server)
	r1s          map[int]*R1              // R1 messages received from servers (index: server)
	r2s          map[int]*R2              // R2 messages received from servers (index: server)
	polyCommit   map[int][]abstract.Point // Commitments of server polynomials (index: server)
	secret       map[int][]int            // Valid shares per secret/server (source server index -> list of target server indices)
	chosenSecret map[int][]int            // Chosen secrets contributing to collective randomness

	// Misc
	Done        chan bool // Channel to signal the end of a protocol run
	SecretReady bool      // Boolean to indicate whether the collect randomness is ready or not

	//Byzantine map[int]int // for simulating byzantine servers (= key)
}

// Share encapsulates all information for encrypted or decrypted shares and the
// respective consistency proofs.
type Share struct {
	Source int            // Source server index
	Target int            // Target server index
	Pos    int            // Share position
	Val    abstract.Point // Share value
	Proof  ProofCore      // ZK-verification proof
}

// Transcript represents the record of a protocol run created by the client.
type Transcript struct {
	SID          []byte             // Session identifier
	Nodes        int                // Total number of nodes (client + server)
	Groups       int                // Number of groups
	Faulty       int                // Maximum number of Byzantine servers
	Purpose      string             // Purpose of protocol run
	Time         time.Time          // Timestamp of initiation
	CliRand      []byte             // Client-chosen randomness (for initial sharding)
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

// I1 is the message sent by the client to the servers in step 1.
type I1 struct {
	Sig       crypto.SchnorrSig // Schnorr signature
	SID       []byte            // Session identifier
	Threshold int               // Secret sharing threshold
	Group     []uint32          // Group indices
	Key       []abstract.Point  // Public keys of trustees
}

// R1 is the reply sent by the servers to the client in step 2.
type R1 struct {
	Sig        crypto.SchnorrSig // Schnorr signature
	HI1        []byte            // Hash of I1
	EncShare   []Share           // Encrypted shares
	CommitPoly []byte            // Marshalled commitment polynomial
}

// I2 is the message sent by the client to the servers in step 3.
type I2 struct {
	Sig          crypto.SchnorrSig // Schnorr signature
	SID          []byte            // Session identifier
	ChosenSecret [][]uint32        // Chosen secrets
	EncShare     []Share           // Encrypted shares
	PolyCommit   []abstract.Point  // Polynomial commitments
}

// R2 is the reply sent by the servers to the client in step 4.
type R2 struct {
	Sig      crypto.SchnorrSig // Schnorr signature
	HI2      []byte            // Hash of I2
	DecShare []Share           // Decrypted shares
}

// WI1 is a SDA-wrapper around I1.
type WI1 struct {
	*sda.TreeNode
	I1
}

// WR1 is a SDA-wrapper around R1.
type WR1 struct {
	*sda.TreeNode
	R1
}

// WI2 is a SDA-wrapper around I2.
type WI2 struct {
	*sda.TreeNode
	I2
}

// WR2 is a SDA-wrapper around R2.
type WR2 struct {
	*sda.TreeNode
	R2
}

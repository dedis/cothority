package protocol

import (
	"sync"
	"time"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/cosi"
	"gopkg.in/dedis/crypto.v0/share/pvss"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	types := []interface{}{
		I1{}, R1{},
		I2{}, R2{},
		I3{}, R3{},
		WI1{}, WR1{},
		WI2{}, WR2{},
		WI3{}, WR3{},
	}
	for _, p := range types {
		network.RegisterMessage(p)
	}
}

// RandHound is the main protocol struct and implements the
// onet.ProtocolInstance interface.
type RandHound struct {
	*onet.TreeNodeInstance                         // The tree node instance of the client / server
	*Session                                       // Session information (client and servers)
	*Messages                                      // Message information (client only)
	mutex                  sync.Mutex              // An awesome mutex!
	Done                   chan bool               // Channel to signal the end of a protocol run
	SecretReady            bool                    // Boolean to indicate whether the collect randomness is ready or not
	cosi                   *cosi.CoSi              // Collective signing instance
	commits                map[int]abstract.Point  // Commits for collective signing
	chosenSecrets          []uint32                // Chosen secrets contributing to collective randomness
	records                map[int]map[int]*Record // Records with shares of chosen PVSS secrets; format: [source][target]*Record
	statement              []byte                  // Statement to be collectively signed
	cosig                  []byte                  // Collective signature on statement
	participants           []int                   // Servers participating in collective signing
}

// Session contains all the information necessary for a RandHound run.
type Session struct {
	nodes      int                // Total number of nodes (client and servers)
	groups     int                // Number of groups
	purpose    string             // Purpose of protocol run
	time       time.Time          // Timestamp of protocol initiation
	seed       []byte             // Client-chosen seed for sharding
	clientKey  abstract.Point     // Client public key
	servers    [][]*onet.TreeNode // Grouped servers
	serverKeys [][]abstract.Point // Grouped server keys
	indices    [][]int            // Grouped server indices
	thresholds []uint32           // Grouped thresholds
	groupNum   map[int]int        // Mapping of roster server index to group number
	groupPos   map[int]int        // Mapping of roster server index to position in the group
	sid        []byte             // Session identifier
}

// Messages stores all the messages the client collects during a RandHound run.
type Messages struct {
	i1  *I1         // I1 message sent to servers
	i2s map[int]*I2 // I2 messages sent to servers (index: server)
	i3  *I3         // I3 message sent to servers
	r1s map[int]*R1 // R1 messages received from servers (index: server)
	r2s map[int]*R2 // R2 messages received from servers (index: server)
	r3s map[int]*R3 // R3 messages received from servers (index: server)
}

// Record stores related encrypted and decrypted PVSS shares together with the
// commitment.
type Record struct {
	Eval     abstract.Point    // Commitment of polynomial evaluation
	EncShare *pvss.PubVerShare // Encrypted verifiable share
	DecShare *pvss.PubVerShare // Decrypted verifiable share
}

// Share contains information on public verifiable shares and the source and
// target servers.
type Share struct {
	Source      int               // Source roster index
	Target      int               // Target roster index
	PubVerShare *pvss.PubVerShare // Public verifiable share
}

// TODO: Do we need to store the public commitment polynomials in the transcript?
// NOTE: We already store the evaluations of the polynomials in the records.

// Transcript represents the record of a protocol run created by the client.
type Transcript struct {
	Nodes      int                     // Total number of nodes (client + server)
	Groups     int                     // Number of groups
	Purpose    string                  // Purpose of protocol run
	Time       time.Time               // Timestamp of protocol initiation
	Seed       []byte                  // Client-chosen seed for sharding
	Keys       []abstract.Point        // Public keys (client + server)
	Thresholds []uint32                // Grouped secret sharing thresholds
	SID        []byte                  // Session identifier
	CoSig      []byte                  // Collective signature on chosen secrets
	Records    map[int]map[int]*Record // Records containing chosen PVSS shares; format: [source][target]*Record
}

// I1 is the message sent by the client to the servers in step 1.
type I1 struct {
	Sig     []byte    // Schnorr signature
	SID     []byte    // Session identifier
	Groups  int       // Number of groups
	Seed    []byte    // Sharding seed
	Purpose string    // Purpose of protocol run
	Time    time.Time // Timestamp of protocol initiation
}

// R1 is the reply sent by the servers to the client in step 2.
type R1 struct {
	Sig       []byte           // Schnorr signature
	SID       []byte           // Session identifier
	HI1       []byte           // Hash of I1
	EncShares []*Share         // Encrypted shares
	Coeffs    []abstract.Point // Commitments to polynomial coefficients
	V         abstract.Point   // Server commitment used to sign chosen secrets
}

// I2 is the message sent by the client to the servers in step 3.
type I2 struct {
	Sig           []byte           // Schnorr signature
	SID           []byte           // Session identifier
	ChosenSecrets []uint32         // Chosen secrets
	EncShares     []*Share         // Encrypted PVSS shares
	Evals         []abstract.Point // Commitments of polynomial evaluations
	C             abstract.Scalar  // Challenge used to sign chosen secrets
}

// R2 is the reply sent by the servers to the client in step 4.
type R2 struct {
	Sig []byte          // Schnorr signature
	SID []byte          // Session identifier
	HI2 []byte          // Hash of I2
	R   abstract.Scalar // Response used to sign chosen secrets
}

// I3 is the message sent by the client to the servers in step 5.
type I3 struct {
	Sig   []byte // Schnorr signature
	SID   []byte // Session identifier
	CoSig []byte // Collective signature on chosen secrets
}

// R3 is the reply sent by the servers to the client in step 6.
type R3 struct {
	Sig       []byte   // Schnorr signature
	SID       []byte   // Session identifier
	HI3       []byte   // Hash of I3
	DecShares []*Share // Decrypted PVSS shares
}

// WI1 is a onet-wrapper around I1.
type WI1 struct {
	*onet.TreeNode
	I1
}

// WR1 is a onet-wrapper around R1.
type WR1 struct {
	*onet.TreeNode
	R1
}

// WI2 is a onet-wrapper around I2.
type WI2 struct {
	*onet.TreeNode
	I2
}

// WR2 is a onet-wrapper around R2.
type WR2 struct {
	*onet.TreeNode
	R2
}

// WI3 is a onet-wrapper around I3.
type WI3 struct {
	*onet.TreeNode
	I3
}

// WR3 is a onet-wrapper around R3.
type WR3 struct {
	*onet.TreeNode
	R3
}

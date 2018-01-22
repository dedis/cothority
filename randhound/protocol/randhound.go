package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share/pvss"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
)

// Name can be used to refer to the protool name
var Name = "RandHound"

// Suite in randhound needs the group and a XOF.
type Suite interface {
	kyber.Group
	kyber.XOFFactory
	kyber.Encoding
	kyber.Random
	kyber.HashFactory
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
	commits                map[int]kyber.Point     // Commits for collective signing (index: source)
	chosenSecrets          []uint32                // Chosen secrets contributing to collective randomness
	records                map[int]map[int]*Record // Records with shares of chosen PVSS secrets; format: [source][target]*Record
	statement              []byte                  // Statement to be collectively signed
	cosig                  []byte                  // Collective signature on statement
	cosi                   cosiVars                // all cosi-variables needed during the operation
}

type cosiVars struct {
	v    kyber.Scalar // Commitment - private part
	V    kyber.Point  // Commitment - public part
	c    kyber.Scalar // Challenge
	r    kyber.Scalar // Response
	mask *cosi.Mask   // Mask of who signed
}

// Session contains all the information necessary for a RandHound run.
type Session struct {
	nodes      int                // Total number of nodes (client and servers)
	groups     int                // Number of groups
	purpose    string             // Purpose of protocol run
	time       int64              // Timestamp of protocol initiation, as seconds from January 1, 1970 UTC
	seed       []byte             // Client-chosen seed for sharding
	clientKey  kyber.Point        // Client public key
	servers    [][]*onet.TreeNode // Grouped servers
	serverKeys [][]kyber.Point    // Grouped server keys
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
	Eval     kyber.Point       // Commitment of polynomial evaluation
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
	Time       int64                   // Timestamp of protocol initiation, as seconds since January 1, 1970 UTC
	Seed       []byte                  // Client-chosen seed for sharding
	Keys       []kyber.Point           // Public keys (client + server)
	Thresholds []uint32                // Grouped secret sharing thresholds
	SID        []byte                  // Session identifier
	CoSig      []byte                  // Collective signature on chosen secrets
	Records    map[int]map[int]*Record // Records containing chosen PVSS shares; format: [source][target]*Record
}

func init() {
	onet.GlobalProtocolRegister(Name, NewRandHound)
}

// NewRandHound generates a new RandHound instance.
func NewRandHound(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	rh := &RandHound{
		TreeNodeInstance: node,
	}
	h := []interface{}{
		rh.handleI1, rh.handleI2, rh.handleI3,
		rh.handleR1, rh.handleR2, rh.handleR3,
	}
	err := rh.RegisterHandlers(h...)
	return rh, err
}

// Setup initializes a RandHound instance on client-side and sets some basic
// parameters. Needs to be called before Start.
func (rh *RandHound) Setup(nodes int, groups int, purpose string) error {
	var err error

	// Setup session information
	if rh.Session, err = rh.newSession(nodes, groups, purpose, time.Now().Unix(), nil, rh.Public()); err != nil {
		return err
	}

	// Setup message buffers
	rh.Messages = rh.newMessages()

	// Setup CoSi instance
	// rh.cosi = cosi.NewCosi(rh.Suite(), rh.Private(), rh.Roster().Publics())

	// Setup other stuff
	rh.records = make(map[int]map[int]*Record)
	rh.commits = make(map[int]kyber.Point)
	rh.Done = make(chan bool, 1)
	rh.SecretReady = false

	return nil
}

// Start initiates the RandHound protocol run. The client pseudorandomly
// chooses the server grouping, forms an I1 message for each group, and sends
// it to all servers of that group.
func (rh *RandHound) Start() error {
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Setup first message
	rh.i1 = &I1{
		SID:     rh.sid,
		Groups:  rh.groups,
		Seed:    rh.seed,
		Purpose: rh.purpose,
		Time:    rh.time,
	}

	// Sign first message
	if err := signSchnorr(rh.Suite(), rh.Private(), rh.i1); err != nil {
		return err
	}

	// Broadcast message to servers which process it as shown in handleI1(...).
	if errs := rh.Broadcast(rh.i1); errs != nil {
		return errs[0]
	}
	return nil
}

// Shard uses the seed to produce a pseudorandom permutation of the numbers of
// 1,...,n-1 and splits the result into s shards.
func Shard(suite kyber.XOFFactory, seed []byte, n, s int) ([][]int, error) {
	if n == 0 || s == 0 || n < s {
		return nil, fmt.Errorf("number of requested shards not supported")
	}

	// Compute a random permutation of [1,...,n-1]
	m := rand.Perm(n - 1)
	for i := range m {
		m[i]++
	}

	// Create sharding of the current roster according to the above permutation
	sharding := make([][]int, s)
	for i, j := range m {
		sharding[i%s] = append(sharding[i%s], j)
	}

	return sharding, nil
}

// Random creates the collective randomness from the shares and the protocol
// transcript.
func (rh *RandHound) Random() ([]byte, *Transcript, error) {
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	if !rh.SecretReady {
		return nil, nil, errors.New("secret not recoverable")
	}

	// Recover randomness
	rb, err := recoverRandomness(rh.Suite(), rh.sid, rh.Roster().Publics(), rh.thresholds, rh.indices, rh.records)
	if err != nil {
		return nil, nil, err
	}

	// Setup transcript
	transcript := &Transcript{
		Nodes:      rh.nodes,
		Groups:     rh.groups,
		Purpose:    rh.purpose,
		Time:       rh.time,
		Seed:       rh.seed,
		Keys:       rh.Roster().Publics(),
		Thresholds: rh.thresholds,
		SID:        rh.sid,
		CoSig:      rh.cosig,
		Records:    rh.records,
	}

	return rb, transcript, nil
}

// Verify checks a given collective random string against its protocol transcript.
func Verify(suite Suite, random []byte, t *Transcript) error {
	//rh.mutex.Lock()
	//defer rh.mutex.Unlock()

	// Recover the sharding
	indices, err := Shard(suite, t.Seed, t.Nodes, t.Groups)
	if err != nil {
		return err
	}

	clientKey := t.Keys[0] // NOTE: we assume for now that the client key is always at index 0
	serverKeys := make([][]kyber.Point, t.Groups)
	for i, group := range indices {
		k := make([]kyber.Point, len(group))
		for j, g := range group {
			k[j] = t.Keys[g]
		}
		serverKeys[i] = k
	}

	// Verify session identifier
	sid, err := sessionID(suite, clientKey, serverKeys, indices, t.Purpose, t.Time)
	if err != nil {
		return err
	}

	if !bytes.Equal(t.SID, sid) {
		return fmt.Errorf("wrong session identifier")
	}

	// Recover chosen secrets from records
	chosenSecrets := chosenSecrets(t.Records)

	// Recover statement = SID || chosen secrets
	statement := new(bytes.Buffer)
	if _, err := statement.Write(t.SID); err != nil {
		return err
	}
	for _, cs := range chosenSecrets {
		binary.Write(statement, binary.LittleEndian, cs)
	}

	// Verify collective signature on statement
	if err := cosi.Verify(suite, t.Keys, statement.Bytes(), t.CoSig, cosi.CompletePolicy{}); err != nil {
		return err
	}

	// Recover randomness
	rb, err := recoverRandomness(suite, t.SID, t.Keys, t.Thresholds, indices, t.Records)
	if err != nil {
		return err
	}

	if !bytes.Equal(random, rb) {
		return errors.New("bad randomness")
	}

	return nil
}

// Suite returns our randhound suite from the cothority-suite
func (rh *RandHound) Suite() Suite {
	return cothority.Suite.(Suite)
}

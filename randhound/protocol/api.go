package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/cosi"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
)

func init() {
	onet.GlobalProtocolRegister("RandHound", NewRandHound)
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

	location, _ := time.LoadLocation("Europe/Vienna")
	time := time.Now().In(location)

	// Setup session information
	if rh.Session, err = rh.newSession(nodes, groups, purpose, time, nil, rh.Public()); err != nil {
		return err
	}

	// Setup message buffers
	rh.Messages = rh.newMessages()

	// Setup CoSi instance
	rh.cosi = cosi.NewCosi(rh.Suite(), rh.Private(), rh.Roster().Publics())

	// Setup other stuff
	rh.records = make(map[int]map[int]*Record)
	rh.commits = make(map[int]abstract.Point)
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
	if err := rh.Broadcast(rh.i1); err != nil {
		return err
	}
	return nil
}

// Shard uses the seed to produce a pseudorandom permutation of the numbers of
// 1,...,n-1, and to group them into s shards.
func Shard(suite abstract.Suite, seed []byte, n, s int) ([][]int, error) {
	if n == 0 || s == 0 || n < s {
		return nil, fmt.Errorf("number of requested shards not supported")
	}

	// Compute a random permutation of [1,...,n-1]
	prng := suite.Cipher(seed)
	m := make([]int, n-1)
	for i := range m {
		j := int(random.Uint64(prng) % uint64(i+1))
		m[i] = m[j]
		m[j] = i + 1
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
func Verify(suite abstract.Suite, random []byte, t *Transcript) error {
	//rh.mutex.Lock()
	//defer rh.mutex.Unlock()

	// Recover the sharding
	indices, err := Shard(suite, t.Seed, t.Nodes, t.Groups)
	if err != nil {
		return err
	}

	clientKey := t.Keys[0] // NOTE: we assume for now that the client key is always at index 0
	serverKeys := make([][]abstract.Point, t.Groups)
	for i, group := range indices {
		k := make([]abstract.Point, len(group))
		for j, g := range group {
			k[j] = t.Keys[g]
		}
		serverKeys[i] = k
	}

	// Fix time zone
	loc, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		return err
	}

	// Verify session identifier
	sid, err := sessionID(suite, clientKey, serverKeys, indices, t.Purpose, t.Time.In(loc))
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
	if err := cosi.VerifySignature(suite, t.Keys, statement.Bytes(), t.CoSig); err != nil {
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

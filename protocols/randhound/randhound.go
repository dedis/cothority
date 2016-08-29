package randhound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

// RandHound ...
type RandHound struct {
	*sda.TreeNodeInstance
	Transcript    *Transcript       // Transcript of a protocol run
	Done          chan bool         // To signal that a protocol run is finished
	mutex         sync.Mutex        // ...
	counter       uint32            // XXX: dummy, remove later
	Server        [][]*sda.TreeNode // Grouped servers (with connection infos)
	ServerToGroup []int             // Server to group mapping
	NumR1s        []int             // Number of valid R1 messages arrived per group
	NumR2s        []int             // Number of valid R2 messages arrived per group
}

// Transcript ...
type Transcript struct {
	SID     []byte      // Session identifier
	Session *Session    // Session parameters
	R1s     map[int]*R1 // R1 messages received from servers
	R2s     map[int]*R2 // R2 messages received from servers
}

// Session ...
type Session struct {
	Nodes   uint32    // Total number of nodes (client + servers)
	Faulty  uint32    // Maximum number of Byzantine servers
	Purpose string    // Purpose of the protocol run
	Time    time.Time // Timestamp of initiation
	Rand    []byte    // Client-chosen randomness
	Group   []*Group  // Server grouping
}

// Group ...
type Group struct {
	Threshold int              // Secret sharing threshold
	Idx       []int            // Global indices of servers
	Key       []abstract.Point // Public keys of servers
}

// I1 message
type I1 struct {
	SID       []byte           // Session identifier
	Threshold int              // Secret sharing threshold
	Key       []abstract.Point // Public keys of trustees
}

// R1 message
type R1 struct {
	HI1      []byte           // Hash of I1
	SX       []abstract.Point // Encrypted Shares
	EncProof []ProofCore      // Encryption consistency proofs
	PolyBin  []byte           // Marshalled commitment polynomial
}

// I2 message
type I2 struct {
	SID       []byte           // Session identifier
	SX        []abstract.Point // Encrypted shares
	EncProof  []ProofCore      // Encryption consistency proofs
	PolyBin   [][]byte         // Marshalled commitment polynomials
	Threshold int              // Secret sharing threshold XXX: probably remove later
	Idx       int              // Index of the server within the group
}

// R2 message
type R2 struct {
	HI2      []byte           // Hash of I2
	S        []abstract.Point // Decrypted shares
	DecProof []ProofCore      // Decryption consistency proofs
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

// NewRandHound ...
func NewRandHound(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	// Setup RandHound protocol struct
	rh := &RandHound{
		TreeNodeInstance: node,
		counter:          0,
	}

	// Setup message handlers
	h := []interface{}{
		rh.handleI1, rh.handleI2,
		rh.handleR1, rh.handleR2,
	}
	err := rh.RegisterHandlers(h...)

	return rh, err
}

// Setup ...
func (rh *RandHound) Setup(nodes uint32, faulty uint32, groups uint32, purpose string) error {

	rh.Transcript = &Transcript{}
	rh.Transcript.Session = &Session{
		Nodes:   nodes,
		Faulty:  faulty,
		Purpose: purpose,
		Group:   make([]*Group, groups),
	}
	rh.Transcript.R1s = make(map[int]*R1, nodes)
	rh.Transcript.R2s = make(map[int]*R2, nodes)

	rh.NumR1s = make([]int, groups)
	rh.NumR2s = make([]int, groups)

	rh.Done = make(chan bool, 1)
	rh.counter = 0

	return nil
}

// SID ...
func (rh *RandHound) SID() ([]byte, error) {

	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.LittleEndian, rh.Transcript.Session.Nodes); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, rh.Transcript.Session.Faulty); err != nil {
		return nil, err
	}

	//if err := binary.Write(buf, binary.LittleEndian, uint32(len(rh.Transcript.Session.Group))); err != nil {
	//	return nil, err
	//}

	if _, err := buf.WriteString(rh.Transcript.Session.Purpose); err != nil {
		return nil, err
	}

	t, err := rh.Transcript.Session.Time.MarshalBinary()
	if err != nil {
		return nil, err
	}

	if _, err := buf.Write(t); err != nil {
		return nil, err
	}

	if _, err := buf.Write(rh.Transcript.Session.Rand); err != nil {
		return nil, err
	}

	for _, group := range rh.Transcript.Session.Group {

		// Write threshold
		if err := binary.Write(buf, binary.LittleEndian, uint32(group.Threshold)); err != nil {
			return nil, err
		}

		// Write keys
		for _, k := range group.Key {
			kb, err := k.MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := buf.Write(kb); err != nil {
				return nil, err
			}
		}

		// XXX: Write indices?
	}

	return crypto.HashBytes(rh.Suite().Hash(), buf.Bytes())
}

// Start ...
func (rh *RandHound) Start() error {

	// Set timestamp
	rh.Transcript.Session.Time = time.Now()

	// Choose randomness
	hs := rh.Suite().Hash().Size()
	rand := make([]byte, hs)
	random.Stream.XORKeyStream(rand, rand)
	rh.Transcript.Session.Rand = rand

	// Determine server grouping
	serverGroup, keyGroup, err := rh.Shard(rand, uint32(len(rh.Transcript.Session.Group)))
	if err != nil {
		return err
	}
	rh.Server = serverGroup

	rh.ServerToGroup = make([]int, rh.Transcript.Session.Nodes)
	for i, group := range serverGroup {

		g := &Group{
			Threshold: 2 * len(keyGroup[i]) / 3,
			Idx:       make([]int, len(group)),
			Key:       keyGroup[i],
		}

		for j, s := range group {
			g.Idx[j] = s.ServerIdentityIdx            // Record server indices that belong to this group
			rh.ServerToGroup[s.ServerIdentityIdx] = i // Record group the server belongs to
		}

		rh.Transcript.Session.Group[i] = g
	}

	// Determine session identifier
	sid, err := rh.SID()
	if err != nil {
		return err
	}
	rh.Transcript.SID = sid

	for i, group := range serverGroup {

		// Send messages to servers
		i1 := &I1{
			SID:       sid,
			Threshold: rh.Transcript.Session.Group[i].Threshold,
			Key:       rh.Transcript.Session.Group[i].Key,
		}

		if err := rh.Multicast(i1, group...); err != nil {
			return err
		}
	}
	return nil
}

func (rh *RandHound) handleI1(i1 WI1) error {

	msg := &i1.I1
	//log.Lvlf1("RandHound - I1: %v\n", rh.index())

	// Map SID to base point H
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))

	// Init PVSS
	pvss := NewPVSS(rh.Suite(), H, msg.Threshold)
	sX, encProof, pb, err := pvss.Split(msg.Key, nil)
	if err != nil {
		return err
	}

	// Send message back to client
	hi1 := []byte{1} // XXX: compute hash of I1
	r1 := &R1{HI1: hi1, SX: sX, EncProof: encProof, PolyBin: pb}
	//log.Lvlf1("RandHound - I1: %v\n%v\n", rh.index(), encProof)
	return rh.SendTo(rh.Root(), r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {

	msg := &r1.R1
	//log.Lvlf1("RandHound - R1: %v\n", rh.index())

	idx := r1.ServerIdentityIdx
	grp := rh.ServerToGroup[idx]

	// Map SID to base point H
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.Transcript.SID))

	// Init PVSS
	pvss := NewPVSS(rh.Suite(), H, rh.Transcript.Session.Group[grp].Threshold)

	// Verify encryption consistency proof
	n := len(rh.Transcript.Session.Group[grp].Key)
	pbx := make([][]byte, n)
	index := make([]int, n)
	for i := 0; i < n; i++ {
		pbx[i] = msg.PolyBin
		index[i] = i
	}
	sH, err := pvss.Commits(pbx, index) // XXX: buffer sH!
	if err != nil {
		return err
	}

	f, err := pvss.Verify(H, rh.Transcript.Session.Group[grp].Key, sH, msg.SX, msg.EncProof)
	if err != nil {
		// XXX: We probably shouldn't return an error here. Instead just drop
		// the message and record nil (?) in the transcript and don't increase
		// the NumR1s counter;
		return errors.New(fmt.Sprintf("%v:%v\n", err, f))
	}

	rh.mutex.Lock()
	//defer rh.mutex.Unlock()

	// Record message in the transcript
	// XXX: Spec first records and then verifies a message. What's better?
	rh.Transcript.R1s[idx] = msg
	rh.NumR1s[grp] += 1
	rh.mutex.Unlock()

	// Once enough valid messages from a group are available, continue XXX:
	// Check if a group-wise commitment of the client is sufficient or if we
	// need a global one. The spec currently sends the global commitment to
	// each server. Maybe it's sufficient to send a per-group commitment and
	// just record the global commitment in the transcript. Check!
	if rh.NumR1s[grp] == len(rh.Transcript.Session.Group[grp].Key) {
		sid := rh.Transcript.SID

		// Re-shuffle R1 messages and send them out

		group := rh.Transcript.Session.Group[grp]
		n := len(group.Idx)

		// XXX: i is the local server index *within* the group to which we send
		// the re-shuffled message
		for i := 0; i < n; i++ {

			sX := make([]abstract.Point, n) // buffer!
			encProof := make([]ProofCore, n)
			polyBin := make([][]byte, n)

			//  k is the local server index, j is the global server index
			for k, j := range group.Idx {
				r1 := rh.Transcript.R1s[j]
				sX[k] = r1.SX[i]
				encProof[k] = r1.EncProof[i]
				polyBin[k] = r1.PolyBin
			}

			i2 := &I2{
				SID:       sid,
				SX:        sX,
				EncProof:  encProof,
				PolyBin:   polyBin, // XXX: send only (buffered) sH later
				Threshold: rh.Transcript.Session.Group[grp].Threshold,
				Idx:       i,
			}

			if err := rh.SendTo(rh.Server[grp][i], i2); err != nil {
				return err
			}

		}
	}
	return nil
}

func (rh *RandHound) handleI2(i2 WI2) error {

	msg := &i2.I2
	//log.Lvlf1("RandHound - I2: %v\n", rh.index())

	// Map SID to base point H
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))

	// Init PVSS
	pvss := NewPVSS(rh.Suite(), H, msg.Threshold)

	// Verify encryption consistency proof
	n := len(msg.SX)
	pbx := make([][]byte, n)
	index := make([]int, n)
	X := make([]abstract.Point, n)
	x := make([]abstract.Scalar, n)
	for i := 0; i < n; i++ {
		pbx[i] = msg.PolyBin[i]
		index[i] = msg.Idx
		X[i] = rh.Public()
		x[i] = rh.Private()
	}
	sH, err := pvss.Commits(pbx, index)
	if err != nil {
		return err
	}

	f, err := pvss.Verify(H, X, sH, msg.SX, msg.EncProof)
	if err != nil {
		//log.Lvlf1("RandHound - I2 - Verification failed: %v %v %v\n", rh.index(), f, err)
		// XXX: We probably shouldn't return an error here. Instead just drop
		// the message and record nil (?) in the transcript and don't increase
		// the NumR1s counter;
		return errors.New(fmt.Sprintf("%v:%v\n", err, f))
	}
	//log.Lvlf1("RandHound - I2 - Encryption verification passed: %v\n", rh.Index())

	// Decrypt shares
	shares, decProof, err := pvss.Reveal(rh.Private(), msg.SX)

	hi2 := []byte{3}
	r2 := &R2{HI2: hi2, S: shares, DecProof: decProof}
	return rh.SendTo(rh.Root(), r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {

	msg := &r2.R2

	idx := r2.ServerIdentityIdx  // global server index
	grp := rh.ServerToGroup[idx] // group

	n := len(msg.S)
	X := make([]abstract.Point, n)
	sX := make([]abstract.Point, n)

	group := rh.Transcript.Session.Group[grp]
	for i := 0; i < n; i++ {
		X[i] = r2.ServerIdentity.Public
	}

	i := 0
	for k, j := range group.Idx {
		if j == idx {
			i = k
		}
	}

	for k, j := range group.Idx {
		r1 := rh.Transcript.R1s[j]
		sX[k] = r1.SX[i]
	}

	// Map SID to base point H
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.Transcript.SID))

	// Init PVSS
	pvss := NewPVSS(rh.Suite(), H, rh.Transcript.Session.Group[grp].Threshold)
	_ = pvss

	e, err := pvss.Verify(rh.Suite().Point().Base(), msg.S, X, sX, msg.DecProof)
	if err != nil {
		//log.Lvlf1("RandHound - R2 - Verification failed: %v %v %v\n", idx, e, err)
		// XXX: We probably shouldn't return an error here. Instead just drop
		// the message and record nil (?) in the transcript and don't increase
		// the NumR1s counter;
		return errors.New(fmt.Sprintf("%v:%v\n", err, e))
	}
	//log.Lvlf1("RandHound - R2 - Verification passed: %v\n", idx)

	//log.Lvlf1("RandHound - R2: %v %v\n", rh.index(), &r2.R2)

	rh.mutex.Lock()
	// Record message in the transcript
	// XXX: Spec first records and then verifies a message. What's better?
	rh.Transcript.R2s[idx] = msg
	rh.NumR2s[grp] += 1
	rh.counter++
	//log.Lvlf1("%v %v\n", idx, rh.Transcript.R2s[idx].S)
	rh.mutex.Unlock()

	//log.Lvlf1("RandHound - R2 - Counter: %v %v\n", rh.index(), rh.counter)
	if rh.counter == rh.Transcript.Session.Nodes-1 {

		secret := rh.Suite().Point().Null()
		for _, group := range rh.Transcript.Session.Group {
			pvss := NewPVSS(rh.Suite(), H, group.Threshold)
			_ = pvss

			for _, j := range group.Idx {
				_ = j
				//log.Lvlf1("%v\n", rh.Transcript.R2s[j].S)
				ps, err := pvss.Recover(rh.Transcript.R2s[j].S)
				if err != nil {
					return err
				}
				//log.Lvlf1("%v\n", ps)
				secret = rh.Suite().Point().Add(secret, ps)
			}
		}

		log.Lvlf1("RandHound - Public Random String: %v\n", secret)

		rh.Done <- true
	}
	return nil
}

func (rh *RandHound) index() uint32 {
	return uint32(rh.Index())
}

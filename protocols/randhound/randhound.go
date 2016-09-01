package randhound

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

// TODO:
// - Client commitments to the final list of secrets that will be used for the
//	 randomness; currently simply all secrets are used
// - Create transcript
// - Verify transcript
// - Handling of failing encryption/decryption proofs
// - Sane conditions on client-side when to proceed
// - Import / export transcript in JSON
// - When handling R1 client-side, maybe store encrypted shares in a sorted way for later...
// - There seems to be still a bug in the sharding function that does not
//	 assign all nodes to the groups

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

// RandHound ...
type RandHound struct {
	*sda.TreeNodeInstance

	// Session information
	Nodes   int       // Total number of nodes (client + servers)
	Faulty  int       // Maximum number of Byzantine servers
	Purpose string    // Purpose of the protocol run
	Time    time.Time // Timestamp of initiation
	Rand    []byte    // Client-chosen randomness
	SID     []byte    // Session identifier
	Group   []*Group  // Server grouping

	// Auxiliary information
	ServerIdxToGroupNum []int // Mapping of gloabl server index to group number
	ServerIdxToGroupIdx []int // Mapping of global server index to group server index

	// For signaling the end of a protocol run
	Done chan bool

	// XXX: Dummy, remove later
	counter int
}

// Group ...
type Group struct {
	Server    []*sda.TreeNode          // Servers of the group
	Threshold int                      // Secret sharing threshold
	Idx       []int                    // Global indices of servers (= ServerIdentityIdx)
	Key       []abstract.Point         // Public keys of servers
	HI1       []byte                   // Hash of I1 message
	HI2       [][]byte                 // Hashes of I2 messages
	R1s       map[int]*R1              // R1 messages received from servers
	R2s       map[int]*R2              // R2 messages received from servers
	Commit    map[int][]abstract.Point // Commitments of server polynomials
	mutex     sync.Mutex
}

// Transcript ...
//type Transcript struct {
//	Random []byte             // Collective randomness
//	Key    [][]abstract.Point // Grouped public keys
//	Index  [][]int            // Grouped server indices
//}

// I1 message
type I1 struct {
	SID       []byte           // Session identifier
	Threshold int              // Secret sharing threshold
	Key       []abstract.Point // Public keys of trustees
}

// R1 message
type R1 struct {
	HI1        []byte           // Hash of I1
	EncShare   []abstract.Point // Encrypted shares
	EncProof   []ProofCore      // Encryption consistency proofs
	CommitPoly []byte           // Marshalled commitment polynomial
}

// I2 message
type I2 struct {
	SID      []byte           // Session identifier
	EncShare []abstract.Point // Encrypted shares
	EncProof []ProofCore      // Encryption consistency proofs
	Commit   []abstract.Point // Polynomial commitments
}

// R2 message
type R2 struct {
	HI2      []byte           // Hash of I2
	DecShare []abstract.Point // Decrypted shares
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
func (rh *RandHound) Setup(nodes int, faulty int, groups int, purpose string) error {

	rh.Nodes = nodes
	rh.Faulty = faulty
	rh.Purpose = purpose
	rh.Group = make([]*Group, groups)
	rh.Done = make(chan bool, 1)
	rh.counter = 0

	return nil
}

// SessionID ...
func (rh *RandHound) SessionID() ([]byte, error) {

	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.LittleEndian, uint32(rh.Nodes)); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, uint32(rh.Faulty)); err != nil {
		return nil, err
	}

	//if err := binary.Write(buf, binary.LittleEndian, uint32(len(rh.Group))); err != nil {
	//	return nil, err
	//}

	if _, err := buf.WriteString(rh.Purpose); err != nil {
		return nil, err
	}

	t, err := rh.Time.MarshalBinary()
	if err != nil {
		return nil, err
	}

	if _, err := buf.Write(t); err != nil {
		return nil, err
	}

	if _, err := buf.Write(rh.Rand); err != nil {
		return nil, err
	}

	for _, group := range rh.Group {

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

// Shard produces a pseudorandom sharding of the network entity list
// based on a seed and a number of requested shards.
func (rh *RandHound) Shard(seed []byte, shards int) ([][]*sda.TreeNode, [][]abstract.Point, error) {

	nodes := rh.Nodes

	if shards == 0 || nodes < shards {
		return nil, nil, fmt.Errorf("Number of requested shards not supported")
	}

	// Compute a random permutation of [1,n-1]
	prng := rh.Suite().Cipher(seed)
	m := make([]uint32, nodes-1)
	for i := range m {
		j := int(random.Uint64(prng) % uint64(i+1))
		m[i] = m[j]
		m[j] = uint32(i) + 1
	}

	// Create sharding of the current Roster according to the above permutation
	el := rh.List()
	n := int(nodes/shards) + 1
	sharding := [][]*sda.TreeNode{}
	shard := []*sda.TreeNode{}
	keys := [][]abstract.Point{}
	k := []abstract.Point{}
	for i, j := range m {
		shard = append(shard, el[j])
		k = append(k, el[j].ServerIdentity.Public)
		if (i%n == n-1) || (i == len(m)-1) {
			sharding = append(sharding, shard)
			shard = make([]*sda.TreeNode, 0)
			keys = append(keys, k)
			k = make([]abstract.Point, 0)
		}
	}

	log.Lvlf1("%v", m)

	// Ensure that the last shard has at least two elements
	if shards > 1 && len(keys[shards-1]) == 1 {
		l := len(sharding[shards-2])
		x := sharding[shards-2][l-1]
		y := keys[shards-2][l-1]
		sharding[shards-1] = append(sharding[shards-1], x)
		keys[shards-1] = append(keys[shards-1], y)
		sharding[shards-2] = sharding[shards-2][:l-1]
		keys[shards-2] = keys[shards-2][:l-1]
	}

	return sharding, keys, nil
}

// Start ...
func (rh *RandHound) Start() error {

	// Set timestamp
	rh.Time = time.Now()

	// Choose randomness
	hs := rh.Suite().Hash().Size()
	rand := make([]byte, hs)
	random.Stream.XORKeyStream(rand, rand)
	rh.Rand = rand

	// Determine server grouping
	serverGroup, keyGroup, err := rh.Shard(rand, len(rh.Group))
	if err != nil {
		return err
	}

	rh.ServerIdxToGroupNum = make([]int, rh.Nodes)
	rh.ServerIdxToGroupIdx = make([]int, rh.Nodes)
	for i, group := range serverGroup {

		g := &Group{
			Server:    group,
			Threshold: 2 * len(keyGroup[i]) / 3,
			Idx:       make([]int, len(group)),
			Key:       keyGroup[i],
			HI1:       make([]byte, 0),
			HI2:       make([][]byte, len(group)),
			R1s:       make(map[int]*R1),
			R2s:       make(map[int]*R2),
			Commit:    make(map[int][]abstract.Point),
		}

		for j, server := range group {
			g.Idx[j] = server.ServerIdentityIdx                  // Record global server indices (=ServerIdentityIdx) that belong to this group
			rh.ServerIdxToGroupNum[server.ServerIdentityIdx] = i // Record group number the server belongs to
			rh.ServerIdxToGroupIdx[server.ServerIdentityIdx] = j // Record the group index of the server
		}

		rh.Group[i] = g

		log.Lvlf1("%v %v", g.Idx, len(g.Idx))
	}

	sid, err := rh.SessionID()
	if err != nil {
		return err
	}
	rh.SID = sid

	for i, group := range serverGroup {

		i1 := &I1{
			SID:       sid,
			Threshold: rh.Group[i].Threshold,
			Key:       rh.Group[i].Key,
		}

		i1b, err := network.MarshalRegisteredType(i1)
		if err != nil {
			return err
		}

		rh.Group[i].HI1, err = crypto.HashBytes(rh.Suite().Hash(), i1b)
		if err != nil {
			return err
		}

		if err := rh.Multicast(i1, group...); err != nil {
			return err
		}
	}
	return nil
}

func (rh *RandHound) handleI1(i1 WI1) error {

	msg := &i1.I1

	// Init PVSS and create shares
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))
	pvss := NewPVSS(rh.Suite(), H, msg.Threshold)
	encShare, encProof, pb, err := pvss.Split(msg.Key, nil)
	if err != nil {
		return err
	}

	i1b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	hi1, err := crypto.HashBytes(rh.Suite().Hash(), i1b)
	if err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), &R1{HI1: hi1, EncShare: encShare, EncProof: encProof, CommitPoly: pb})
}

func (rh *RandHound) handleR1(r1 WR1) error {

	msg := &r1.R1
	//log.Lvlf1("RandHound - R1: %v\n", rh.index())

	idx := r1.ServerIdentityIdx
	grp := rh.ServerIdxToGroupNum[idx]

	if !bytes.Equal(rh.Group[grp].HI1, msg.HI1) {
		return fmt.Errorf("Server %v of group %v replied to the wrong I1 message", idx, grp)
	}

	n := len(rh.Group[grp].Key)
	pbx := make([][]byte, n)
	index := make([]int, n)
	for i := 0; i < n; i++ {
		pbx[i] = msg.CommitPoly
		index[i] = i
	}

	// Init PVSS and recover commits
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.SID))
	pvss := NewPVSS(rh.Suite(), H, rh.Group[grp].Threshold)
	commit, err := pvss.Commits(pbx, index)
	if err != nil {
		return err
	}

	// Verify encrypted shares
	f, err := pvss.Verify(H, rh.Group[grp].Key, commit, msg.EncShare, msg.EncProof)
	if err != nil {
		// Erase invalid data
		for i := range f {
			commit[i] = nil
			msg.EncShare[i] = nil
			msg.EncProof[i] = ProofCore{}
		}
	}

	// Record commit and message
	rh.Group[grp].mutex.Lock()
	rh.Group[grp].Commit[idx] = commit
	rh.Group[grp].R1s[idx] = msg
	rh.Group[grp].mutex.Unlock()

	// Continue once "enough" R1 messages have been collected
	if len(rh.Group[grp].R1s) == len(rh.Group[grp].Key) {

		n := len(rh.Group[grp].Idx)
		for i := 0; i < n; i++ {

			// Collect all shares, proofs, and commits intended for server i
			encShare := make([]abstract.Point, n)
			encProof := make([]ProofCore, n)
			commit := make([]abstract.Point, n)

			//  j is the group server index, k is the global server index
			for j, k := range rh.Group[grp].Idx {
				r1 := rh.Group[grp].R1s[k]
				encShare[j] = r1.EncShare[i]
				encProof[j] = r1.EncProof[i]
				commit[j] = rh.Group[grp].Commit[k][i]
			}

			i2 := &I2{
				SID:      rh.SID,
				EncShare: encShare,
				EncProof: encProof,
				Commit:   commit,
			}

			i2b, err := network.MarshalRegisteredType(i2)
			if err != nil {
				return err
			}

			rh.Group[grp].HI2[i], err = crypto.HashBytes(rh.Suite().Hash(), i2b)
			if err != nil {
				return err
			}

			if err := rh.SendTo(rh.Group[grp].Server[i], i2); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rh *RandHound) handleI2(i2 WI2) error {

	msg := &i2.I2
	//log.Lvlf1("RandHound - I2: %v\n", rh.index())

	// Prepare data
	n := len(msg.EncShare)
	X := make([]abstract.Point, n)
	x := make([]abstract.Scalar, n)
	for i := 0; i < n; i++ {
		X[i] = rh.Public()
		x[i] = rh.Private()
	}

	// Init PVSS and verify encryption consistency proof
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))
	pvss := NewPVSS(rh.Suite(), H, 0)
	f, err := pvss.Verify(H, X, msg.Commit, msg.EncShare, msg.EncProof)
	if err != nil {
		// Erase invalid data
		for i := range f {
			msg.Commit[i] = nil
			msg.EncShare[i] = nil
			msg.EncProof[i] = ProofCore{}
		}
	}
	//log.Lvlf1("RandHound - I2 - Encryption verification passed: %v\n", rh.Index())

	// Decrypt shares
	decShare, decProof, err := pvss.Reveal(rh.Private(), msg.EncShare)
	if err != nil {
		return err
	}

	i2b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	hi2, err := crypto.HashBytes(rh.Suite().Hash(), i2b)
	if err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), &R2{HI2: hi2, DecShare: decShare, DecProof: decProof})
}

func (rh *RandHound) handleR2(r2 WR2) error {

	msg := &r2.R2

	idx := r2.ServerIdentityIdx
	grp := rh.ServerIdxToGroupNum[idx]

	if !bytes.Equal(rh.Group[grp].HI2[rh.ServerIdxToGroupIdx[idx]], msg.HI2) {
		return fmt.Errorf("Server %v of group %v replied to the wrong I2 message", idx, grp)
	}

	n := len(msg.DecShare)
	X := make([]abstract.Point, n)
	encShare := make([]abstract.Point, n)

	group := rh.Group[grp]
	for i := 0; i < n; i++ {
		X[i] = r2.ServerIdentity.Public
	}

	// Get encrypted shares intended for server idx
	i := rh.ServerIdxToGroupIdx[idx]
	for j, k := range group.Idx {
		r1 := rh.Group[grp].R1s[k]
		encShare[j] = r1.EncShare[i]
	}

	// Init PVSS and verify shares
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.SID))
	pvss := NewPVSS(rh.Suite(), H, rh.Group[grp].Threshold)
	f, err := pvss.Verify(rh.Suite().Point().Base(), msg.DecShare, X, encShare, msg.DecProof)
	if err != nil {
		// Erase invalid data
		for i := range f {
			msg.DecShare[i] = nil
			msg.DecProof[i] = ProofCore{}
		}
	}

	rh.Group[grp].mutex.Lock()
	rh.Group[grp].R2s[idx] = msg
	rh.counter++
	rh.Group[grp].mutex.Unlock()

	// Continue once "enough" R2 messages have been collected
	// XXX: this check should be replaced by a more sane one
	if rh.counter == rh.Nodes-1 {

		rnd := rh.Suite().Point().Null()
		for i, group := range rh.Group {
			pvss := NewPVSS(rh.Suite(), H, group.Threshold)

			for _, j := range group.Idx {
				ps, err := pvss.Recover(rh.Group[i].R2s[j].DecShare)
				if err != nil {
					return err
				}
				rnd = rh.Suite().Point().Add(rnd, ps)
			}
		}

		rb, err := rnd.MarshalBinary()
		if err != nil {
			return err
		}

		log.Lvlf1("RandHound - collective randomness: %v", rb)

		rh.Done <- true
	}
	return nil
}

// CreateTranscript ...
func CreateTranscript() {}

// VerifyTranscript ...
func VerifyTranscript() {}

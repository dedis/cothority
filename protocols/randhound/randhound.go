package randhound

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
// - Signatures of I-messages are currently not checked by the servers since
//	 the latter are assumed to be stateless; should they know the public key of the client?

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
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
	rh.Groups = groups
	rh.Faulty = faulty
	rh.Purpose = purpose
	rh.Threshold = make([]int, groups)
	rh.HashI1 = make(map[int][]byte)
	rh.HashI2 = make(map[int][]byte)
	rh.R1s = make(map[int]*R1)
	rh.R2s = make(map[int]*R2)
	rh.CR1 = make([]int, groups)
	rh.CR2 = make([]int, groups)
	rh.Commit = make(map[int][]abstract.Point)
	rh.ServerIdxToGroupNum = make([]int, nodes)
	rh.ServerIdxToGroupIdx = make([]int, nodes)
	rh.Done = make(chan bool, 1)
	rh.counter = 0

	return nil
}

// Start ...
func (rh *RandHound) Start() error {

	var err error

	// Set timestamp
	rh.Time = time.Now()

	// Choose client randomness
	hs := rh.Suite().Hash().Size()
	rand := make([]byte, hs)
	random.Stream.XORKeyStream(rand, rand)
	rh.CliRand = rand

	// Determine server grouping
	rh.Server, rh.Key, err = rh.Shard(rand, rh.Groups)
	if err != nil {
		return err
	}

	for i, group := range rh.Server {
		rh.Threshold[i] = 2 * len(group) / 3
		rh.Commit[i] = make([]abstract.Point, len(group))
		for j, server := range group {
			l := server.ServerIdentityIdx
			rh.ServerIdxToGroupNum[l] = i
			rh.ServerIdxToGroupIdx[l] = j
		}
	}

	rh.SID, err = rh.sessionID(rh.Nodes, rh.Faulty, rh.Purpose, rh.Time, rh.CliRand, rh.Threshold, rh.Key)
	if err != nil {
		return err
	}

	for i, group := range rh.Server {

		i1 := &I1{
			Sig:       crypto.SchnorrSig{},
			SID:       rh.SID,
			Threshold: rh.Threshold[i],
			Key:       rh.Key[i],
		}

		i1b, err := network.MarshalRegisteredType(i1)
		if err != nil {
			return err
		}

		rh.HashI1[i], err = crypto.HashBytes(rh.Suite().Hash(), i1b)
		if err != nil {
			return err
		}

		i1.Sig, err = crypto.SignSchnorr(rh.Suite(), rh.Private(), i1b)
		if err != nil {
			return err
		}

		if err := rh.Multicast(i1, group...); err != nil {
			return err
		}
	}
	return nil
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
	m := make([]int, nodes-1)
	for i := range m {
		j := int(random.Uint64(prng) % uint64(i+1))
		m[i] = m[j]
		m[j] = i + 1
	}

	// Create sharding of the current Roster according to the above permutation
	el := rh.List()
	sharding := make([][]*sda.TreeNode, shards)
	keys := make([][]abstract.Point, shards)
	for i, j := range m {
		sharding[i%shards] = append(sharding[i%shards], el[j])
		keys[i%shards] = append(keys[i%shards], el[j].ServerIdentity.Public)
	}

	return sharding, keys, nil
}

// CreateTranscript ...
func (rh *RandHound) CreateTranscript() Transcript {

	return Transcript{
		SID:       rh.SID,
		Nodes:     rh.Nodes,
		Faulty:    rh.Faulty,
		Time:      rh.Time,
		CliRand:   rh.CliRand,
		Threshold: rh.Threshold,
		Key:       rh.Key,
	}
}

// VerifyTranscript ...
func (rh *RandHound) VerifyTranscript() bool {

	return true
}

func (rh *RandHound) sessionID(nodes int, faulty int, purpose string, time time.Time, rand []byte, threshold []int, key [][]abstract.Point) ([]byte, error) {

	buf := new(bytes.Buffer)

	if len(threshold) != len(key) {
		return nil, fmt.Errorf("Non-matching number of group thresholds and keys")
	}

	if err := binary.Write(buf, binary.LittleEndian, uint32(nodes)); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, uint32(faulty)); err != nil {
		return nil, err
	}

	//if err := binary.Write(buf, binary.LittleEndian, uint32(len(rh.Group))); err != nil {
	//	return nil, err
	//}

	if _, err := buf.WriteString(purpose); err != nil {
		return nil, err
	}

	t, err := time.MarshalBinary()
	if err != nil {
		return nil, err
	}

	if _, err := buf.Write(t); err != nil {
		return nil, err
	}

	if _, err := buf.Write(rand); err != nil {
		return nil, err
	}

	for _, t := range threshold {
		if err := binary.Write(buf, binary.LittleEndian, uint32(t)); err != nil {
			return nil, err
		}
	}

	for _, gk := range key {
		for _, k := range gk {
			kb, err := k.MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := buf.Write(kb); err != nil {
				return nil, err
			}
		}
	}

	return crypto.HashBytes(rh.Suite().Hash(), buf.Bytes())
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

	msg.Sig = crypto.SchnorrSig{} // XXX: hack
	i1b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	hi1, err := crypto.HashBytes(rh.Suite().Hash(), i1b)
	if err != nil {
		return err
	}

	r1 := &R1{
		Sig:        crypto.SchnorrSig{}, // XXX: hack
		HI1:        hi1,
		EncShare:   encShare,
		EncProof:   encProof,
		CommitPoly: pb,
	}

	r1b, err := network.MarshalRegisteredType(r1)
	if err != nil {
		return err
	}

	r1.Sig, err = crypto.SignSchnorr(rh.Suite(), rh.Private(), r1b)
	if err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {

	msg := &r1.R1

	idx := r1.ServerIdentityIdx
	grp := rh.ServerIdxToGroupNum[idx]
	i := rh.ServerIdxToGroupIdx[idx]

	// Verify R1 message signature
	sig := msg.Sig
	msg.Sig = crypto.SchnorrSig{} // XXX: hack
	msgb, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	if err := crypto.VerifySchnorr(rh.Suite(), rh.Key[grp][i], msgb, sig); err != nil {
		return err
	}

	if !bytes.Equal(rh.HashI1[grp], msg.HI1) {
		return fmt.Errorf("Server %v of group %v replied to the wrong I1 message", idx, grp)
	}

	n := len(rh.Key[grp])
	pbx := make([][]byte, n)
	index := make([]int, n)
	for i := 0; i < n; i++ {
		pbx[i] = msg.CommitPoly
		index[i] = i
	}

	// Init PVSS and recover commits
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.SID))
	pvss := NewPVSS(rh.Suite(), H, rh.Threshold[grp])
	commit, err := pvss.Commits(pbx, index)
	if err != nil {
		return err
	}

	// Verify encrypted shares
	f, err := pvss.Verify(H, rh.Key[grp], commit, msg.EncShare, msg.EncProof)
	if err != nil {
		// Erase invalid data
		for i := range f {
			commit[i] = nil
			msg.EncShare[i] = nil
			msg.EncProof[i] = ProofCore{}
		}
	}

	// Record commit and message
	rh.mutex.Lock()
	rh.R1s[idx] = msg
	rh.CR1[grp]++
	rh.Commit[idx] = commit
	rh.mutex.Unlock()

	// Continue once "enough" R1 messages have been collected
	if rh.CR1[grp] == len(rh.Server[grp]) {

		n := len(rh.Server[grp])
		for i, target := range rh.Server[grp] {

			// Collect all shares, proofs, and commits intended for server i
			encShare := make([]abstract.Point, n)
			encProof := make([]ProofCore, n)
			commit := make([]abstract.Point, n)

			//  j is the group server index, k is the global server index
			for j, server := range rh.Server[grp] {
				k := server.ServerIdentityIdx
				r1 := rh.R1s[k]
				encShare[j] = r1.EncShare[i]
				encProof[j] = r1.EncProof[i]
				commit[j] = rh.Commit[k][i]
			}

			i2 := &I2{
				Sig:      crypto.SchnorrSig{},
				SID:      rh.SID,
				EncShare: encShare,
				EncProof: encProof,
				Commit:   commit,
			}

			i2b, err := network.MarshalRegisteredType(i2)
			if err != nil {
				return err
			}

			rh.HashI2[target.ServerIdentityIdx], err = crypto.HashBytes(rh.Suite().Hash(), i2b)
			if err != nil {
				return err
			}

			i2.Sig, err = crypto.SignSchnorr(rh.Suite(), rh.Private(), i2b)
			if err != nil {
				return err
			}

			if err := rh.SendTo(target, i2); err != nil {
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

	msg.Sig = crypto.SchnorrSig{} // XXX: hack
	i2b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	hi2, err := crypto.HashBytes(rh.Suite().Hash(), i2b)
	if err != nil {
		return err
	}

	r2 := &R2{
		Sig:      crypto.SchnorrSig{}, // XXX: hack
		HI2:      hi2,
		DecShare: decShare,
		DecProof: decProof,
	}

	r2b, err := network.MarshalRegisteredType(r2)
	if err != nil {
		return err
	}

	r2.Sig, err = crypto.SignSchnorr(rh.Suite(), rh.Private(), r2b)
	if err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {

	//XXX: continue here
	msg := &r2.R2

	idx := r2.ServerIdentityIdx
	grp := rh.ServerIdxToGroupNum[idx]
	i := rh.ServerIdxToGroupIdx[idx]

	// Verify R2 message signature
	sig := msg.Sig
	msg.Sig = crypto.SchnorrSig{} // XXX: hack
	msgb, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	if err := crypto.VerifySchnorr(rh.Suite(), rh.Key[grp][i], msgb, sig); err != nil {
		return err
	}

	if !bytes.Equal(rh.HashI2[idx], msg.HI2) {
		return fmt.Errorf("Server %v of group %v replied to the wrong I2 message", idx, grp)
	}

	n := len(msg.DecShare)
	X := make([]abstract.Point, n)
	encShare := make([]abstract.Point, n)

	//group := rh.Group[grp]
	for i := 0; i < n; i++ {
		X[i] = r2.ServerIdentity.Public // XXX: get it from the local cache
	}

	// Get encrypted shares intended for server idx
	for j, server := range rh.Server[grp] {
		k := server.ServerIdentityIdx
		r1 := rh.R1s[k]
		encShare[j] = r1.EncShare[i]
	}

	// Init PVSS and verify shares
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.SID))
	pvss := NewPVSS(rh.Suite(), H, rh.Threshold[grp])
	f, err := pvss.Verify(rh.Suite().Point().Base(), msg.DecShare, X, encShare, msg.DecProof)
	if err != nil {
		// Erase invalid data
		for i := range f {
			msg.DecShare[i] = nil
			msg.DecProof[i] = ProofCore{}
		}
	}

	rh.mutex.Lock()
	rh.R2s[idx] = msg
	rh.CR2[grp]++
	rh.counter++
	rh.mutex.Unlock()

	// Continue once "enough" R2 messages have been collected
	// XXX: this check should be replaced by a more sane one
	if rh.counter == rh.Nodes-1 {

		rnd := rh.Suite().Point().Null()

		for i, group := range rh.Server {
			pvss := NewPVSS(rh.Suite(), H, rh.Threshold[i])

			for _, server := range group {
				j := server.ServerIdentityIdx
				ps, err := pvss.Recover(rh.R2s[j].DecShare)
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

package randhound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
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
// - Client public key should go into SID

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
	rh.I1s = make(map[int]*I1)
	rh.I2s = make(map[int]*I2)
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

	// Set some group parameters
	for i, group := range rh.Server {
		rh.Threshold[i] = 2 * len(group) / 3
		rh.Commit[i] = make([]abstract.Point, len(group))
		for j, server := range group {
			l := server.ServerIdentityIdx
			rh.ServerIdxToGroupNum[l] = i
			rh.ServerIdxToGroupIdx[l] = j
		}
	}

	// Comptue session id
	rh.SID, err = rh.sessionID(rh.Nodes, rh.Faulty, rh.Purpose, rh.Time, rh.CliRand, rh.Threshold, rh.Key)
	if err != nil {
		return err
	}

	// Multicast first message to servers
	for i, group := range rh.Server {

		i1 := &I1{
			SID:       rh.SID,
			Threshold: rh.Threshold[i],
			Key:       rh.Key[i],
		}

		rh.mutex.Lock()

		// Sign I1 and store signature in i1.Sig
		if err := signSchnorr(rh.Suite(), rh.Private(), i1); err != nil {
			return err
		}

		rh.I1s[i] = i1

		rh.mutex.Unlock()

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

	// Create sharding of the current roster according to the above permutation
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
func (rh *RandHound) CreateTranscript() *Transcript {

	t := &Transcript{
		SID:       rh.SID,
		Nodes:     rh.Nodes,
		Groups:    rh.Groups,
		Faulty:    rh.Faulty,
		Purpose:   rh.Purpose,
		Time:      rh.Time,
		CliRand:   rh.CliRand,
		CliKey:    rh.Public(),
		Threshold: rh.Threshold,
		Key:       rh.Key,
	}

	index := make([][]int, rh.Groups)
	i1s := make([]*I1, rh.Groups)
	i2s := make([]*I2, rh.Nodes)

	// Index 0 (=client) in r1s and r2s is always empty
	r1s := make([]*R1, rh.Nodes)
	r2s := make([]*R2, rh.Nodes)

	for i, group := range rh.Server {
		idx := make([]int, len(group))
		for j, server := range group {
			k := server.ServerIdentityIdx
			idx[j] = k
			i2s[k] = rh.I2s[k]
			r1s[k] = rh.R1s[k]
			r2s[k] = rh.R2s[k]
		}
		index[i] = idx
		i1s[i] = rh.I1s[i]
	}

	t.Index = index
	t.I1s = i1s
	t.I2s = i2s
	t.R1s = r1s
	t.R2s = r2s

	return t
}

// VerifyTranscript ...
func (rh *RandHound) VerifyTranscript(suite abstract.Suite, t *Transcript) error {

	// Verify SID
	sid, err := rh.sessionID(t.Nodes, t.Faulty, t.Purpose, t.Time, t.CliRand, t.Threshold, t.Key)
	if err != nil {
		return err
	}

	if !bytes.Equal(t.SID, sid) {
		return fmt.Errorf("Wrong session identifier")
	}

	for i, grp := range t.Index {

		// Verify I1 signatures
		if err := verifySchnorr(suite, t.CliKey, t.I1s[i]); err != nil {
			return err
		}

		for j, k := range grp {

			// Verify R1 signatures
			if err := verifySchnorr(suite, t.Key[i][j], t.R1s[k]); err != nil {
				return err
			}

			// Verify I2 signatures
			if err := verifySchnorr(suite, t.CliKey, t.I2s[k]); err != nil {
				return err
			}

			// Verify R2 signatures
			if err := verifySchnorr(suite, t.Key[i][j], t.R2s[k]); err != nil {
				return err
			}
		}
	}

	// Verify proofs

	// Compare randomness

	return nil
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
		HI1:        hi1,
		EncShare:   encShare,
		EncProof:   encProof,
		CommitPoly: pb,
	}

	// Sign R1 and store signature in R1.Sig
	if err := signSchnorr(rh.Suite(), rh.Private(), r1); err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {

	msg := &r1.R1

	idx := r1.ServerIdentityIdx
	grp := rh.ServerIdxToGroupNum[idx]
	i := rh.ServerIdxToGroupIdx[idx]

	rh.mutex.Lock()
	// Verify R1 message signature
	if err := verifySchnorr(rh.Suite(), rh.Key[grp][i], msg); err != nil {
		return err
	}

	// Verify that server replied to the correct I1 message
	if err := verifyMessage(rh.Suite(), rh.I1s[grp], msg.HI1); err != nil {
		return err
	}
	rh.mutex.Unlock()

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

			// Collect all shares, proofs, and commits intended for target server
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

			rh.mutex.Lock()
			if err := signSchnorr(rh.Suite(), rh.Private(), i2); err != nil {
				return err
			}

			rh.I2s[target.ServerIdentityIdx] = i2
			rh.mutex.Unlock()

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
		HI2:      hi2,
		DecShare: decShare,
		DecProof: decProof,
	}

	// Sign R2 and store signature in R2.Sig
	if err := signSchnorr(rh.Suite(), rh.Private(), r2); err != nil {
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

	rh.mutex.Lock()
	// Verify R2 message signature
	if err := verifySchnorr(rh.Suite(), rh.Key[grp][i], msg); err != nil {
		return err
	}

	// Verify that server replied to the correct I1 message
	if err := verifyMessage(rh.Suite(), rh.I2s[idx], msg.HI2); err != nil {
		return err
	}
	rh.mutex.Unlock()

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

func signSchnorr(suite abstract.Suite, key abstract.Scalar, m interface{}) error {

	// Reset signature field
	reflect.ValueOf(m).Elem().FieldByName("Sig").Set(reflect.ValueOf(crypto.SchnorrSig{})) // XXX: hack

	// Marshal message
	mb, err := network.MarshalRegisteredType(m)
	if err != nil {
		return err
	}

	// Sign message
	sig, err := crypto.SignSchnorr(suite, key, mb)
	if err != nil {
		return err
	}

	// Store signature
	reflect.ValueOf(m).Elem().FieldByName("Sig").Set(reflect.ValueOf(sig)) // XXX: hack

	return nil
}

func verifySchnorr(suite abstract.Suite, key abstract.Point, m interface{}) error {

	// Make a copy of the signature
	x := reflect.ValueOf(m).Elem().FieldByName("Sig")
	sig := reflect.New(x.Type()).Elem()
	sig.Set(x)

	// Reset signature field
	reflect.ValueOf(m).Elem().FieldByName("Sig").Set(reflect.ValueOf(crypto.SchnorrSig{})) // XXX: hack

	// Marshal message
	mb, err := network.MarshalRegisteredType(m)
	if err != nil {
		return err
	}

	// Copy back original signature
	reflect.ValueOf(m).Elem().FieldByName("Sig").Set(sig) // XXX: hack

	return crypto.VerifySchnorr(suite, key, mb, sig.Interface().(crypto.SchnorrSig))
}

func verifyMessage(suite abstract.Suite, m interface{}, hash1 []byte) error {

	// Make a copy of the signature
	x := reflect.ValueOf(m).Elem().FieldByName("Sig")
	sig := reflect.New(x.Type()).Elem()
	sig.Set(x)

	// Reset signature field
	reflect.ValueOf(m).Elem().FieldByName("Sig").Set(reflect.ValueOf(crypto.SchnorrSig{})) // XXX: hack

	// Marshal ...
	mb, err := network.MarshalRegisteredType(m)
	if err != nil {
		return err
	}

	// ... and hash message
	hash2, err := crypto.HashBytes(suite.Hash(), mb)
	if err != nil {
		return err
	}

	// Copy back original signature
	reflect.ValueOf(m).Elem().FieldByName("Sig").Set(sig) // XXX: hack

	// Compare hashes
	if !bytes.Equal(hash1, hash2) {
		return errors.New("Message has a different hash than the given one")
	}

	return nil
}

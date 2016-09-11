package randhound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

// TODO:
// - Client commitments to the final list of secrets that will be used for the
//	 randomness; currently simply all secrets are used
// - Handle failing encryption/decryption proofs
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
	rh.Group = make([][]int, groups)
	rh.Threshold = make([]int, groups)
	rh.I1s = make(map[int]*I1)
	rh.I2s = make(map[int]*I2)
	rh.R1s = make(map[int]*R1)
	rh.R2s = make(map[int]*R2)
	rh.PolyCommit = make(map[int][]abstract.Point)
	rh.Secret = make(map[int][]int)
	rh.ChosenSecret = make(map[int][]int)
	rh.ServerIdxToGroupNum = make([]int, nodes)
	rh.ServerIdxToGroupIdx = make([]int, nodes)
	rh.Done = make(chan bool, 1)
	rh.SecretReady = false

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
		rh.PolyCommit[i] = make([]abstract.Point, len(group))
		g := make([]int, len(group))
		for j, server0 := range group {
			s0 := server0.ServerIdentityIdx
			rh.ServerIdxToGroupNum[s0] = i
			rh.ServerIdxToGroupIdx[s0] = j
			g[j] = s0
		}
		rh.Group[i] = g
	}

	// Comptue session id
	rh.SID, err = rh.sessionID(rh.Nodes, rh.Faulty, rh.Purpose, rh.Time, rh.CliRand, rh.Threshold, rh.Public(), rh.Key)
	if err != nil {
		return err
	}

	// Multicast first message to servers
	for i, group := range rh.Server {

		index := make([]uint32, len(group))
		for j, server := range group {
			index[j] = uint32(server.ServerIdentityIdx)
		}

		i1 := &I1{
			SID:       rh.SID,
			Threshold: rh.Threshold[i],
			Group:     index,
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

	if shards == 0 || rh.Nodes < shards {
		return nil, nil, fmt.Errorf("Number of requested shards not supported")
	}

	// Compute a random permutation of [1,n-1]
	prng := rh.Suite().Cipher(seed)
	m := make([]int, rh.Nodes-1)
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

// Random ...
func (rh *RandHound) Random() ([]byte, *Transcript, error) {

	if !rh.SecretReady {
		return nil, nil, errors.New("Secret not (yet) recoverable")
	}

	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.SID))
	rnd := rh.Suite().Point().Null()

	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Gather all valid shares for a given server
	for source, target := range rh.Secret { // XXX: why iterate over Secret and not ChosenSecret?

		//log.Lvlf1("%v: %v", source, target)
		var share []abstract.Point
		var pos []int
		for _, t := range target {
			r2 := rh.R2s[t]
			for _, s := range r2.DecShare {
				if s.Source == source {
					share = append(share, s.Val)
					pos = append(pos, s.Gen)
				}
			}
		}

		grp := rh.ServerIdxToGroupNum[source]
		pvss := NewPVSS(rh.Suite(), H, rh.Threshold[grp])
		ps, err := pvss.Recover(pos, share, len(rh.Server[grp]))
		if err != nil {
			return nil, nil, err
		}
		rnd = rh.Suite().Point().Add(rnd, ps)
		//log.Lvlf1("Random: %v %v", source, ps)
	}

	rb, err := rnd.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	return rb, rh.CreateTranscript(), nil
}

// CreateTranscript ...
func (rh *RandHound) CreateTranscript() *Transcript {

	t := &Transcript{
		SID:          rh.SID,
		Nodes:        rh.Nodes,
		Groups:       rh.Groups,
		Faulty:       rh.Faulty,
		Purpose:      rh.Purpose,
		Time:         rh.Time,
		CliRand:      rh.CliRand,
		CliKey:       rh.Public(),
		Group:        rh.Group,
		Threshold:    rh.Threshold,
		ChosenSecret: rh.ChosenSecret,
		Key:          rh.Key,
		I1s:          rh.I1s,
		I2s:          rh.I2s,
		R1s:          rh.R1s,
		R2s:          rh.R2s,
	}

	return t
}

// VerifyTranscript ...
func (rh *RandHound) VerifyTranscript(suite abstract.Suite, random []byte, t *Transcript) error {

	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Verify SID
	sid, err := rh.sessionID(t.Nodes, t.Faulty, t.Purpose, t.Time, t.CliRand, t.Threshold, t.CliKey, t.Key)
	if err != nil {
		return err
	}

	if !bytes.Equal(t.SID, sid) {
		return fmt.Errorf("Wrong session identifier")
	}

	// Check message signatures
	for i, group := range t.Group {

		// Verify I1 signatures
		if err := verifySchnorr(suite, t.CliKey, t.I1s[i]); err != nil {
			return err
		}

		for j, k := range group {

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

	// Check zero knowledge proofs of chosen secrets
	H, _ := suite.Point().Pick(nil, suite.Cipher(t.SID))

	rnd := suite.Point().Null()
	for _, val := range rh.ChosenSecret {

		for _, src := range val {

			grp := rh.ServerIdxToGroupNum[src]
			r1 := t.R1s[src]

			var poly [][]byte
			var index []int
			var encShare []abstract.Point
			var decShare []abstract.Point
			var decProof []ProofCore
			var X []abstract.Point
			var target []int
			for i := 0; i < len(r1.EncShare); i++ {
				poly = append(poly, r1.CommitPoly)
				index = append(index, r1.EncShare[i].Gen)
				encShare = append(encShare, r1.EncShare[i].Val)
				X = append(X, t.Key[grp][index[i]]) // XXX: could fail if the encShare is not there

				j := r1.EncShare[i].Target
				r2 := t.R2s[j]
				// XXX: there is still an error below
				for k := 0; k < len(r2.DecShare); k++ {
					if r2.DecShare[k].Source == src {
						decShare = append(decShare, r2.DecShare[k].Val)
						decProof = append(decProof, r2.DecProof[k])
						target = append(target, r2.DecShare[k].Target)
					}
				}
			}

			pvss := NewPVSS(suite, H, t.Threshold[grp])
			polyCommit, err := pvss.Commits(poly, index)
			if err != nil {
				return err
			}

			goodEnc, badEnc, err := pvss.Verify(H, t.Key[grp], polyCommit, encShare, r1.EncProof)
			if err != nil {
				return err
			}
			_ = goodEnc
			_ = badEnc

			// remove bad shares from encShare!

			//log.Lvlf1("Enc: %v %v", goodEnc, badEnc)

			goodDec, badDec, err := pvss.Verify(suite.Point().Base(), decShare, X, encShare, decProof)
			if err != nil {
				return err
			}
			_ = goodDec
			_ = badDec

			// remove bad shares from decShare!

			//log.Lvlf1("Dec: %v %v", goodDec, badDec)

			//log.Lvlf1("Dec: %v %v %v", src, len(decShare), target)

			ps, err := pvss.Recover(index, decShare, len(t.Group[grp])) // XXX: could fail when shares are missing
			if err != nil {
				return err
			}
			rnd = rh.Suite().Point().Add(rnd, ps)
			//log.Lvlf1("Transcript: %v %v", src, ps)

		}
	}

	rb, err := rnd.MarshalBinary()
	if err != nil {
		return err
	}

	if !bytes.Equal(random, rb) {
		return errors.New("Random strings do not match")
	}

	return nil
}

func (rh *RandHound) handleI1(i1 WI1) error {

	msg := &i1.I1

	// Compute hash of the client's message
	msg.Sig = crypto.SchnorrSig{} // XXX: hack
	i1b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	hi1, err := crypto.HashBytes(rh.Suite().Hash(), i1b)
	if err != nil {
		return err
	}

	// Find out the server's index (we assume servers are stateless)
	idx := 0
	for i, j := range msg.Group {
		if msg.Key[i].Equal(rh.Public()) {
			idx = int(j)
			break
		}
	}

	// Init PVSS and create shares
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))
	pvss := NewPVSS(rh.Suite(), H, msg.Threshold)
	idxShare, encShare, encProof, pb, err := pvss.Split(msg.Key, nil)
	if err != nil {
		return err
	}

	share := make([]Share, len(encShare))
	for i := 0; i < len(encShare); i++ {
		share[i] = Share{
			Source: idx,
			Target: int(msg.Group[i]),
			Gen:    idxShare[i],
			Val:    encShare[i],
		}
	}

	r1 := &R1{
		HI1:        hi1,
		EncShare:   share,
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
	pos := rh.ServerIdxToGroupIdx[idx]

	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Verify R1 message signature
	if err := verifySchnorr(rh.Suite(), rh.Key[grp][pos], msg); err != nil {
		return err
	}

	// Verify that server replied to the correct I1 message
	if err := verifyMessage(rh.Suite(), rh.I1s[grp], msg.HI1); err != nil {
		return err
	}

	// Record R1 message
	rh.R1s[idx] = msg

	// Prepare data for recovery of polynomial commits and verification of shares
	n := len(msg.EncShare)
	poly := make([][]byte, n)
	index := make([]int, n)
	encShare := make([]abstract.Point, n)
	for i := 0; i < n; i++ {
		poly[i] = msg.CommitPoly
		index[i] = msg.EncShare[i].Gen
		encShare[i] = msg.EncShare[i].Val
	}

	// Init PVSS and recover polynomial commits
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.SID))
	pvss := NewPVSS(rh.Suite(), H, rh.Threshold[grp])
	polyCommit, err := pvss.Commits(poly, index)
	if err != nil {
		return err
	}

	// Record polynomial commits
	rh.PolyCommit[idx] = polyCommit

	// Return, if we already committed to secrets previously
	if len(rh.ChosenSecret) > 0 {
		return nil
	}

	// Verify encrypted shares
	good, _, err := pvss.Verify(H, rh.Key[grp], polyCommit, encShare, msg.EncProof)
	if err != nil {
		return err
	}

	// Record valid encrypted shares per secret/server
	for i := 0; i < len(good); i++ {
		j := good[i]
		src := msg.EncShare[j].Source
		if _, ok := rh.Secret[idx]; !ok {
			rh.Secret[idx] = make([]int, 0)
		}
		rh.Secret[src] = append(rh.Secret[src], msg.EncShare[j].Target)
	}

	// Check if there is at least a threshold number of reconstructable secrets
	// in each group. If yes we proceed to the next phase. Note the
	// double-usage of the threshold which is used to determine if enough valid
	// shares for a single secret are available and if enough secrets for a
	// given group are available
	goodSecret := make(map[int][]int)
	for i, group := range rh.Server {
		var secret []int
		for _, server := range group {
			j := server.ServerIdentityIdx
			if share, ok := rh.Secret[j]; ok && rh.Threshold[i] <= len(share) {
				secret = append(secret, j)
			}
		}
		if rh.Threshold[i] <= len(secret) {
			goodSecret[i] = secret
		}
	}

	//log.Lvlf1("%v", goodSecret)

	// XXX: abort if all servers replied but not enough shares are available to
	// reconstruct enough secrets
	//if len(rh.R2s) == rh.Nodes-1 && !proceed {
	//	return errors.New("Some secrets are not reconstructable")
	//}

	// If there are enough good secrets and we didn't make a commitment before, proceed ...
	if len(goodSecret) == rh.Groups {

		// Reset secret for the next phase (see handleR2)
		rh.Secret = make(map[int][]int)

		// Choose secrets that contribute to collective randomness
		for i := range rh.Server {

			// Randomly select a threshold of secrets for each group in an order preserving way
			var secret []int
			hs := rh.Suite().Hash().Size()
			rand := make([]byte, hs)
			random.Stream.XORKeyStream(rand, rand)
			prng := rh.Suite().Cipher(rand)

			//log.Lvlf1("%v", len(goodSecret[i]), rh.Threshold[i])

			index := make([]int, len(goodSecret[i]))
			for j := 0; j < len(goodSecret[i]); j++ {
				index[j] = j
			}

			for j := 0; j < rh.Threshold[i]; j++ {
				k := int(random.Uint32(prng) % uint32(len(index)))
				secret = append(secret, index[k])
				index = append(index[:k], index[k+1:]...)
			}
			sort.Ints(secret)

			for j := 0; j < len(secret); j++ {
				secret[j] = goodSecret[i][secret[j]]
			}

			rh.ChosenSecret[i] = secret
		}

		//log.Lvlf1("ChosenSecret: %v", rh.ChosenSecret)

		// Transformation of commitments from int to uint32 to avoid protobuff errors
		var chosenSecret [][]uint32
		for i := range rh.ChosenSecret {
			var l []uint32
			for j := range rh.ChosenSecret[i] {
				l = append(l, uint32(rh.ChosenSecret[i][j]))
			}
			chosenSecret = append(chosenSecret, l)
		}

		// Prepare a message for each server of a group and send it
		for i, group := range rh.Server {
			for j, server := range group {

				// Among the chosen secrets collect all valid shares, proofs,
				// and polynomial commits intended for target server
				var encShare []Share
				var encProof []ProofCore
				var polyCommit []abstract.Point
				for _, k := range rh.ChosenSecret[i] {
					r1 := rh.R1s[k]
					pc := rh.PolyCommit[k]
					encShare = append(encShare, r1.EncShare[j])
					encProof = append(encProof, r1.EncProof[j])
					polyCommit = append(polyCommit, pc[j])
				}

				// XXX: simulate bad data
				//if server.ServerIdentityIdx == 1 || server.ServerIdentityIdx == 2 || server.ServerIdentityIdx == 3 || server.ServerIdentityIdx == 4 || server.ServerIdentityIdx == 5 || server.ServerIdentityIdx == 6 || server.ServerIdentityIdx == 7 || server.ServerIdentityIdx == 8 || server.ServerIdentityIdx == 9 {
				//	bad := []int{0, 1, 2, 3}
				//	for _, b := range bad {
				//		encShare[b].Val = rh.Suite().Point().Null()
				//		log.Lvlf1("R1 - bad enc share: %v %v %v", encShare[b].Source, encShare[b].Target, encShare[b].Gen)
				//	}
				//}

				i2 := &I2{
					Sig:          crypto.SchnorrSig{},
					SID:          rh.SID,
					ChosenSecret: chosenSecret,
					EncShare:     encShare,
					EncProof:     encProof,
					PolyCommit:   polyCommit,
				}

				if err := signSchnorr(rh.Suite(), rh.Private(), i2); err != nil {
					return err
				}

				rh.I2s[server.ServerIdentityIdx] = i2

				if err := rh.SendTo(server, i2); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (rh *RandHound) handleI2(i2 WI2) error {

	msg := &i2.I2
	//log.Lvlf1("RandHound - I2: %v\n", rh.index())

	// Compute hash of the client's message
	msg.Sig = crypto.SchnorrSig{} // XXX: hack
	i2b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return err
	}

	hi2, err := crypto.HashBytes(rh.Suite().Hash(), i2b)
	if err != nil {
		return err
	}

	// Prepare data
	n := len(msg.EncShare)
	X := make([]abstract.Point, n)
	x := make([]abstract.Scalar, n)
	encShare := make([]abstract.Point, n)
	for i := 0; i < n; i++ {
		X[i] = rh.Public()
		x[i] = rh.Private()
		encShare[i] = msg.EncShare[i].Val
	}

	// Init PVSS and verify encryption consistency proof
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))
	pvss := NewPVSS(rh.Suite(), H, 0)

	//log.Lvlf1("%v %v %v", msg.PolyCommit, msg.EncShare, msg.EncProof)

	good, bad, err := pvss.Verify(H, X, msg.PolyCommit, encShare, msg.EncProof)
	if err != nil {
		return err
	}

	// Remove bad shares
	for i := len(bad) - 1; i >= 0; i-- {
		j := bad[i]
		encShare = append(encShare[:j], encShare[j+1:]...)
	}

	// Decrypt shares
	decShare, decProof, err := pvss.Reveal(rh.Private(), encShare)
	if err != nil {
		return err
	}

	share := make([]Share, len(encShare))
	for i := 0; i < len(encShare); i++ {
		j := good[i]
		share[i] = Share{
			Source: msg.EncShare[j].Source,
			Target: msg.EncShare[j].Target,
			Gen:    msg.EncShare[j].Gen,
			Val:    decShare[i],
		}
	}

	// XXX: simulate bad decryption share
	//if rh.Index() == 1 {
	//	msg.EncShare[1] = rh.Suite().Point().Null()
	//}

	//log.Lvlf1("I2: %v %v %v %v %v", rh.Index(), good, bad, len(decShare), len(decProof))

	r2 := &R2{
		HI2:      hi2,
		DecShare: share,
		DecProof: decProof,
	}

	// Sign R2 and store signature in R2.Sig
	if err := signSchnorr(rh.Suite(), rh.Private(), r2); err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {

	msg := &r2.R2

	idx := r2.ServerIdentityIdx
	grp := rh.ServerIdxToGroupNum[idx]
	pos := rh.ServerIdxToGroupIdx[idx]

	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Verify R2 message signature
	if err := verifySchnorr(rh.Suite(), rh.Key[grp][pos], msg); err != nil {
		return err
	}

	// Verify that server replied to the correct I2 message
	if err := verifyMessage(rh.Suite(), rh.I2s[idx], msg.HI2); err != nil {
		return err
	}

	// Record R2 message
	rh.R2s[idx] = msg

	//log.Lvlf1("R2: %v %v %v %v", idx, len(msg.GoodShare), len(msg.DecShare), len(msg.DecProof))

	//rh.mutex.Unlock()

	// XXX: invalidate shares for which we did not remove a decryption proof (?)

	// Get all valid encrypted shares corresponding to the received decrypted
	// shares and intended for "target" (=idx)
	n := len(msg.DecShare)
	X := make([]abstract.Point, n)
	encShare := make([]abstract.Point, n)
	decShare := make([]abstract.Point, n)
	for i := 0; i < n; i++ {
		src := msg.DecShare[i].Source
		X[i] = r2.ServerIdentity.Public
		encShare[i] = rh.R1s[src].EncShare[pos].Val
		decShare[i] = msg.DecShare[i].Val
		//log.Lvlf1("pos: %v; %v %v %v", pos, msg.DecShare[i].Source, msg.DecShare[i].Target, msg.DecShare[i].Gen)
	}

	// Init PVSS and verify shares
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.SID))
	pvss := NewPVSS(rh.Suite(), H, rh.Threshold[grp])
	good, bad, err := pvss.Verify(rh.Suite().Point().Base(), decShare, X, encShare, msg.DecProof)
	if err != nil {
		return err
	}
	_ = bad
	_ = good

	//log.Lvlf1("R2: %v %v %v", idx, good, bad)

	// Record valid decrypted shares per secret/server
	for i := 0; i < len(good); i++ {
		j := good[i]
		src := msg.DecShare[j].Source
		if _, ok := rh.Secret[src]; !ok {
			rh.Secret[src] = make([]int, 0)
		}
		rh.Secret[src] = append(rh.Secret[src], msg.DecShare[j].Target)
	}

	proceed := true
	for i, group := range rh.ChosenSecret {
		for _, server := range group {
			if len(rh.Secret[server]) < rh.Threshold[i] {
				proceed = false
			}
		}
	}

	if len(rh.R2s) == rh.Nodes-1 && !proceed {
		return errors.New("Some secrets are not reconstructable")
	}

	rh.Counter++

	// XXX: there is still a racing condition somehwere in here; currently it
	// only work if we wait until all messages have arrived
	if proceed && !rh.SecretReady && rh.Counter == rh.Nodes-1 {

		//for i := range rh.ChosenSecret {
		//	for j := range rh.ChosenSecret[i] {
		//		k := rh.ChosenSecret[i][j]
		//		s := rh.Secret[k]
		//		log.Lvlf1("%v %v %v", k, s, len(s))
		//	}
		//}

		rh.SecretReady = true
		rh.Done <- true
	}
	return nil
}

func (rh *RandHound) sessionID(nodes int, faulty int, purpose string, time time.Time, rand []byte, threshold []int, clientKey abstract.Point, serverKey [][]abstract.Point) ([]byte, error) {

	buf := new(bytes.Buffer)

	if len(threshold) != len(serverKey) {
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

	cb, err := clientKey.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(cb); err != nil {
		return nil, err
	}

	for _, t := range threshold {
		if err := binary.Write(buf, binary.LittleEndian, uint32(t)); err != nil {
			return nil, err
		}
	}

	for _, gk := range serverKey {
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

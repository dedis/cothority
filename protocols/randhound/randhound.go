// Package randhound is a client/server protocol for creating public random
// strings in an unbiasable and verifiable way given that a threshold of
// participants is honest. The protocol is driven by the client which scavenges
// the public randomness from the servers over the course of two round-trips.
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
// - Import / export transcript in JSON
// - Signatures of I-messages are currently not checked by the servers since
//	 the latter are assumed to be stateless; should they know the public key of the client?

func init() {
	sda.GlobalProtocolRegister("RandHound", NewRandHound)
}

// NewRandHound generates a new RandHound instance.
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

// Setup configures a RandHound instance on client-side. Needs to be called
// before Start.
func (rh *RandHound) Setup(nodes int, faulty int, groups int, purpose string) error {

	rh.nodes = nodes
	rh.groups = groups
	rh.faulty = faulty
	rh.purpose = purpose

	rh.server = make([][]*sda.TreeNode, groups)
	rh.group = make([][]int, groups)
	rh.threshold = make([]int, groups)
	rh.key = make([][]abstract.Point, groups)
	rh.ServerIdxToGroupNum = make([]int, nodes)
	rh.ServerIdxToGroupIdx = make([]int, nodes)

	rh.i1s = make(map[int]*I1)
	rh.i2s = make(map[int]*I2)
	rh.r1s = make(map[int]*R1)
	rh.r2s = make(map[int]*R2)
	rh.polyCommit = make(map[int][]abstract.Point)
	rh.secret = make(map[int][]int)
	rh.chosenSecret = make(map[int][]int)

	rh.Done = make(chan bool, 1)
	rh.SecretReady = false

	return nil
}

// Start initiates the RandHound protocol run. The client pseudo-randomly
// chooses the server grouping, forms an I1 message for each group, and sends
// it to all servers of that group.
func (rh *RandHound) Start() error {

	var err error

	// Set timestamp
	rh.time = time.Now()

	// Choose client randomness
	rand := random.Bytes(rh.Suite().Hash().Size(), random.Stream)
	rh.cliRand = rand

	// Determine server grouping
	rh.server, rh.key, err = rh.Shard(rand, rh.groups)
	if err != nil {
		return err
	}

	// Set some group parameters
	for i, group := range rh.server {
		rh.threshold[i] = 2 * len(group) / 3
		rh.polyCommit[i] = make([]abstract.Point, len(group))
		g := make([]int, len(group))
		for j, server0 := range group {
			s0 := server0.RosterIndex
			rh.ServerIdxToGroupNum[s0] = i
			rh.ServerIdxToGroupIdx[s0] = j
			g[j] = s0
		}
		rh.group[i] = g
	}

	// Compute session id
	rh.sid, err = rh.sessionID(rh.nodes, rh.faulty, rh.purpose, rh.time, rh.cliRand, rh.threshold, rh.Public(), rh.key)
	if err != nil {
		return err
	}

	// Multicast first message to grouped servers
	for i, group := range rh.server {

		index := make([]uint32, len(group))
		for j, server := range group {
			index[j] = uint32(server.RosterIndex)
		}

		i1 := &I1{
			SID:       rh.sid,
			Threshold: rh.threshold[i],
			Group:     index,
			Key:       rh.key[i],
		}

		rh.mutex.Lock()

		// Sign I1 and store signature in i1.Sig
		if err := signSchnorr(rh.Suite(), rh.Private(), i1); err != nil {
			rh.mutex.Unlock()
			return err
		}

		rh.i1s[i] = i1

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

	if shards == 0 || rh.nodes < shards {
		return nil, nil, fmt.Errorf("Number of requested shards not supported")
	}

	// Compute a random permutation of [1,n-1]
	prng := rh.Suite().Cipher(seed)
	m := make([]int, rh.nodes-1)
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

// Random creates the collective randomness from the shares and the protocol
// transcript.
func (rh *RandHound) Random() ([]byte, *Transcript, error) {

	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	if !rh.SecretReady {
		return nil, nil, errors.New("Secret not recoverable")
	}

	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.sid))
	rnd := rh.Suite().Point().Null()

	// Gather all valid shares for a given server
	for source, target := range rh.secret {

		var share []abstract.Point
		var pos []int
		for _, t := range target {
			r2 := rh.r2s[t]
			for _, s := range r2.DecShare {
				if s.Source == source {
					share = append(share, s.Val)
					pos = append(pos, s.Pos)
				}
			}
		}

		grp := rh.ServerIdxToGroupNum[source]
		pvss := NewPVSS(rh.Suite(), H, rh.threshold[grp])
		ps, err := pvss.Recover(pos, share, len(rh.server[grp]))
		if err != nil {
			return nil, nil, err
		}
		rnd = rh.Suite().Point().Add(rnd, ps)
	}

	rb, err := rnd.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	transcript := &Transcript{
		SID:          rh.sid,
		Nodes:        rh.nodes,
		Groups:       rh.groups,
		Faulty:       rh.faulty,
		Purpose:      rh.purpose,
		Time:         rh.time,
		CliRand:      rh.cliRand,
		CliKey:       rh.Public(),
		Group:        rh.group,
		Threshold:    rh.threshold,
		ChosenSecret: rh.chosenSecret,
		Key:          rh.key,
		I1s:          rh.i1s,
		I2s:          rh.i2s,
		R1s:          rh.r1s,
		R2s:          rh.r2s,
	}

	return rb, transcript, nil
}

// Verify checks a given collective random string against a protocol transcript.
func (rh *RandHound) Verify(suite abstract.Suite, random []byte, t *Transcript) error {

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

	// Verify I1 signatures
	for _, i1 := range t.I1s {
		if err := verifySchnorr(suite, t.CliKey, i1); err != nil {
			return err
		}
	}

	// Verify R1 signatures
	for src, r1 := range t.R1s {
		var key abstract.Point
		for i := range t.Group {
			for j := range t.Group[i] {
				if src == t.Group[i][j] {
					key = t.Key[i][j]
				}
			}
		}
		if err := verifySchnorr(suite, key, r1); err != nil {
			return err
		}
	}

	// Verify I2 signatures
	for _, i2 := range t.I2s {
		if err := verifySchnorr(suite, t.CliKey, i2); err != nil {
			return err
		}
	}

	// Verify R2 signatures
	for src, r2 := range t.R2s {
		var key abstract.Point
		for i := range t.Group {
			for j := range t.Group[i] {
				if src == t.Group[i][j] {
					key = t.Key[i][j]
				}
			}
		}
		if err := verifySchnorr(suite, key, r2); err != nil {
			return err
		}
	}

	// Verify message hashes HI1 and HI2; it is okay if some messages are
	// missing as long as there are enough to reconstruct the chosen secrets
	for i, msg := range t.I1s {
		for _, j := range t.Group[i] {
			if _, ok := t.R1s[j]; ok {
				if err := verifyMessage(suite, msg, t.R1s[j].HI1); err != nil {
					return err
				}
			} else {
				log.Lvlf2("Couldn't find R1 message of server %v", j)
			}
		}
	}

	for i, msg := range t.I2s {
		if _, ok := t.R2s[i]; ok {
			if err := verifyMessage(suite, msg, t.R2s[i].HI2); err != nil {
				return err
			}
		} else {
			log.Lvlf2("Couldn't find R2 message of server %v", i)
		}
	}

	// Verify that all servers received the same client commitment
	for server, msg := range t.I2s {
		for i := range msg.ChosenSecret {
			for j := range msg.ChosenSecret[i] {
				if int(msg.ChosenSecret[i][j]) != t.ChosenSecret[i][j] {
					return fmt.Errorf("Server %v received wrong client commitment", server)
				}
			}
		}
	}

	H, _ := suite.Point().Pick(nil, suite.Cipher(t.SID))
	rnd := suite.Point().Null()
	for i, group := range t.ChosenSecret {

		for _, src := range group {

			var poly [][]byte
			var encPos []int
			var encShare []abstract.Point
			var encProof []ProofCore
			var X []abstract.Point

			var decPos []int
			var decShare []abstract.Point
			var decProof []ProofCore

			// All R1 messages of the chosen secrets should be there
			if _, ok := t.R1s[src]; !ok {
				return errors.New("R1 message not found")
			}
			r1 := t.R1s[src]

			for j := 0; j < len(r1.EncShare); j++ {

				// Check availability of corresponding R2 messages, skip if not there
				target := r1.EncShare[j].Target
				if _, ok := t.R2s[target]; !ok {
					continue
				}

				// Gather data on encrypted shares
				poly = append(poly, r1.CommitPoly)
				encPos = append(encPos, r1.EncShare[j].Pos)
				encShare = append(encShare, r1.EncShare[j].Val)
				encProof = append(encProof, r1.EncShare[j].Proof)
				X = append(X, t.Key[i][r1.EncShare[j].Pos])

				// Gather data on decrypted shares
				r2 := t.R2s[target]
				for k := 0; k < len(r2.DecShare); k++ {
					if r2.DecShare[k].Source == src {
						decPos = append(decPos, r2.DecShare[k].Pos)
						decShare = append(decShare, r2.DecShare[k].Val)
						decProof = append(decProof, r2.DecShare[k].Proof)
					}
				}
			}

			// Remove encrypted shares that do not have a corresponding decrypted share
			j := 0
			for j < len(decPos) {
				if encPos[j] != decPos[j] {
					poly = append(poly[:j], poly[j+1:]...)
					encPos = append(encPos[:j], encPos[j+1:]...)
					encShare = append(encShare[:j], encShare[j+1:]...)
					encProof = append(encProof[:j], encProof[j+1:]...)
					X = append(X[:j], X[j+1:]...)
				} else {
					j++
				}
			}
			// If all of the first values where equal remove trailing data on encrypted shares
			if len(decPos) < len(encPos) {
				l := len(decPos)
				poly = poly[:l]
				encPos = encPos[:l]
				encShare = encShare[:l]
				encProof = encProof[:l]
				X = X[:l]
			}

			pvss := NewPVSS(suite, H, t.Threshold[i])

			// Recover polynomial commits
			polyCommit, err := pvss.Commits(poly, encPos)
			if err != nil {
				return err
			}

			// Check encryption consistency proofs
			goodEnc, badEnc, err := pvss.Verify(H, X, polyCommit, encShare, encProof)
			if err != nil {
				return err
			}
			_ = goodEnc
			_ = badEnc

			// Remove bad values
			for j := len(badEnc) - 1; j >= 0; j-- {
				k := badEnc[j]
				X = append(X[:k], X[k+1:]...)
				encShare = append(encShare[:k], encShare[k+1:]...)
				decShare = append(decShare[:k], decShare[k+1:]...)
				decProof = append(decProof[:k], decProof[k+1:]...)
			}

			// Check decryption consistency proofs
			goodDec, badDec, err := pvss.Verify(suite.Point().Base(), decShare, X, encShare, decProof)
			if err != nil {
				return err
			}
			_ = goodDec
			_ = badDec

			// Remove bad shares
			for j := len(badDec) - 1; j >= 0; j-- {
				k := badDec[j]
				decPos = append(decPos[:k], decPos[k+1:]...)
				decShare = append(decShare[:k], decShare[k+1:]...)
			}

			// Recover secret and add it to the collective random point
			ps, err := pvss.Recover(decPos, decShare, len(t.Group[i]))
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

	if !bytes.Equal(random, rb) {
		return errors.New("Bad randomness")
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
			Pos:    idxShare[i],
			Val:    encShare[i],
			Proof:  encProof[i],
		}
	}

	r1 := &R1{
		HI1:        hi1,
		EncShare:   share,
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

	idx := r1.RosterIndex
	grp := rh.ServerIdxToGroupNum[idx]
	pos := rh.ServerIdxToGroupIdx[idx]

	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Verify R1 message signature
	if err := verifySchnorr(rh.Suite(), rh.key[grp][pos], msg); err != nil {
		return err
	}

	// Verify that server replied to the correct I1 message
	if err := verifyMessage(rh.Suite(), rh.i1s[grp], msg.HI1); err != nil {
		return err
	}

	// Record R1 message
	rh.r1s[idx] = msg

	// Prepare data for recovery of polynomial commits and verification of shares
	n := len(msg.EncShare)
	poly := make([][]byte, n)
	index := make([]int, n)
	encShare := make([]abstract.Point, n)
	encProof := make([]ProofCore, n)
	for i := 0; i < n; i++ {
		poly[i] = msg.CommitPoly
		index[i] = msg.EncShare[i].Pos
		encShare[i] = msg.EncShare[i].Val
		encProof[i] = msg.EncShare[i].Proof
	}

	// Init PVSS and recover polynomial commits
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.sid))
	pvss := NewPVSS(rh.Suite(), H, rh.threshold[grp])
	polyCommit, err := pvss.Commits(poly, index)
	if err != nil {
		return err
	}

	// Record polynomial commits
	rh.polyCommit[idx] = polyCommit

	// Return, if we already committed to secrets previously
	if len(rh.chosenSecret) > 0 {
		return nil
	}

	// Verify encrypted shares
	good, _, err := pvss.Verify(H, rh.key[grp], polyCommit, encShare, encProof)
	if err != nil {
		return err
	}

	// Record valid encrypted shares per secret/server
	for _, g := range good {
		if _, ok := rh.secret[idx]; !ok {
			rh.secret[idx] = make([]int, 0)
		}
		rh.secret[idx] = append(rh.secret[idx], msg.EncShare[g].Target)
	}

	// Check if there is at least a threshold number of reconstructable secrets
	// in each group. If yes proceed to the next phase. Note the double-usage
	// of the threshold which is used to determine if enough valid shares for a
	// single secret are available and if enough secrets for a given group are
	// available
	goodSecret := make(map[int][]int)
	for i, group := range rh.server {
		var secret []int
		for _, server := range group {
			j := server.RosterIndex
			if share, ok := rh.secret[j]; ok && rh.threshold[i] <= len(share) {
				secret = append(secret, j)
			}
		}
		if rh.threshold[i] <= len(secret) {
			goodSecret[i] = secret
		}
	}

	// Proceed, if there are enough good secrets
	if len(goodSecret) == rh.groups {

		// Reset secret for the next phase (see handleR2)
		rh.secret = make(map[int][]int)

		// Choose secrets that contribute to collective randomness
		for i := range rh.server {

			// Randomly remove some secrets so that a threshold of secrets remains
			rand := random.Bytes(rh.Suite().Hash().Size(), random.Stream)
			prng := rh.Suite().Cipher(rand)
			secret := goodSecret[i]
			for j := 0; j < len(secret)-rh.threshold[i]; j++ {
				k := int(random.Uint32(prng) % uint32(len(secret)))
				secret = append(secret[:k], secret[k+1:]...)
			}
			rh.chosenSecret[i] = secret
		}

		log.Lvlf3("Grouping: %v", rh.group)
		log.Lvlf3("ChosenSecret: %v", rh.chosenSecret)

		// Transformation of commitments from int to uint32 to avoid protobuff errors
		var chosenSecret = make([][]uint32, len(rh.chosenSecret))
		for i := range rh.chosenSecret {
			var l []uint32
			for j := range rh.chosenSecret[i] {
				l = append(l, uint32(rh.chosenSecret[i][j]))
			}
			chosenSecret[i] = l
		}

		// Prepare a message for each server of a group and send it
		for i, group := range rh.server {
			for j, server := range group {

				// Among the good secrets chosen previously collect all valid
				// shares, proofs, and polynomial commits intended for the
				// target server
				var encShare []Share
				var polyCommit []abstract.Point
				for _, k := range rh.chosenSecret[i] {
					r1 := rh.r1s[k]
					pc := rh.polyCommit[k]
					encShare = append(encShare, r1.EncShare[j])
					polyCommit = append(polyCommit, pc[j])
				}

				i2 := &I2{
					Sig:          crypto.SchnorrSig{},
					SID:          rh.sid,
					ChosenSecret: chosenSecret,
					EncShare:     encShare,
					PolyCommit:   polyCommit,
				}

				if err := signSchnorr(rh.Suite(), rh.Private(), i2); err != nil {
					return err
				}

				rh.i2s[server.RosterIndex] = i2

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
	encShare := make([]abstract.Point, n)
	encProof := make([]ProofCore, n)
	for i := 0; i < n; i++ {
		X[i] = rh.Public()
		encShare[i] = msg.EncShare[i].Val
		encProof[i] = msg.EncShare[i].Proof
	}

	// Init PVSS and verify encryption consistency proof
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))
	pvss := NewPVSS(rh.Suite(), H, 0)

	good, bad, err := pvss.Verify(H, X, msg.PolyCommit, encShare, encProof)
	if err != nil {
		return err
	}

	// Remove bad shares
	for i := len(bad) - 1; i >= 0; i-- {
		j := bad[i]
		encShare = append(encShare[:j], encShare[j+1:]...)
	}

	// Decrypt good shares
	decShare, decProof, err := pvss.Reveal(rh.Private(), encShare)
	if err != nil {
		return err
	}

	share := make([]Share, len(encShare))
	for i := 0; i < len(encShare); i++ {
		X[i] = rh.Public()
		j := good[i]
		share[i] = Share{
			Source: msg.EncShare[j].Source,
			Target: msg.EncShare[j].Target,
			Pos:    msg.EncShare[j].Pos,
			Val:    decShare[i],
			Proof:  decProof[i],
		}
	}

	r2 := &R2{
		HI2:      hi2,
		DecShare: share,
	}

	// Sign R2 and store signature in R2.Sig
	if err := signSchnorr(rh.Suite(), rh.Private(), r2); err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {

	msg := &r2.R2

	idx := r2.RosterIndex
	grp := rh.ServerIdxToGroupNum[idx]
	pos := rh.ServerIdxToGroupIdx[idx]

	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// If the collective secret is already available, ignore all further incoming messages
	if rh.SecretReady {
		return nil
	}

	// Verify R2 message signature
	if err := verifySchnorr(rh.Suite(), rh.key[grp][pos], msg); err != nil {
		return err
	}

	// Verify that server replied to the correct I2 message
	if err := verifyMessage(rh.Suite(), rh.i2s[idx], msg.HI2); err != nil {
		return err
	}

	// Record R2 message
	rh.r2s[idx] = msg

	// Get all valid encrypted shares corresponding to the received decrypted
	// shares and intended for the target server (=idx)
	n := len(msg.DecShare)
	X := make([]abstract.Point, n)
	encShare := make([]abstract.Point, n)
	decShare := make([]abstract.Point, n)
	decProof := make([]ProofCore, n)
	for i := 0; i < n; i++ {
		src := msg.DecShare[i].Source
		//tgt := msg.DecShare[i].Target
		//X[i] = rh.key[grp][pos] //r2.ServerIdentity.Public
		X[i] = rh.key[grp][pos]
		encShare[i] = rh.r1s[src].EncShare[pos].Val
		decShare[i] = msg.DecShare[i].Val
		decProof[i] = msg.DecShare[i].Proof
	}

	// Init PVSS and verify shares
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.sid))
	pvss := NewPVSS(rh.Suite(), H, rh.threshold[grp])
	good, bad, err := pvss.Verify(rh.Suite().Point().Base(), decShare, X, encShare, decProof)
	if err != nil {
		return err
	}
	_ = bad
	_ = good

	// Record valid decrypted shares per secret/server
	for i := 0; i < len(good); i++ {
		j := good[i]
		src := msg.DecShare[j].Source
		if _, ok := rh.secret[src]; !ok {
			rh.secret[src] = make([]int, 0)
		}
		rh.secret[src] = append(rh.secret[src], msg.DecShare[j].Target)
	}

	proceed := true
	for i, group := range rh.chosenSecret {
		for _, server := range group {
			if len(rh.secret[server]) < rh.threshold[i] {
				proceed = false
			}
		}
	}

	if len(rh.r2s) == rh.nodes-1 && !proceed {
		rh.Done <- true
		return errors.New("Some chosen secrets are not reconstructable")
	}

	if proceed && !rh.SecretReady {
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

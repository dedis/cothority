package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/dedis/onet/log"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/cosi"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/crypto.v0/share/pvss"
)

// Some error definitions.
var errorWrongSession = errors.New("wrong session identifier")

func (rh *RandHound) handleI1(i1 WI1) error {
	msg := &i1.I1
	var err error
	src := i1.RosterIndex
	idx := rh.TreeNode().RosterIndex
	keys := rh.Roster().Publics()
	nodes := len(keys)
	clientKey := keys[src]

	// Verify I1 message signature
	if err := verifySchnorr(rh.Suite(), clientKey, msg); err != nil {
		return err
	}

	// Fix time zone
	loc, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		return err
	}

	// Setup session
	if rh.Session, err = rh.newSession(nodes, msg.Groups, msg.Purpose, msg.Time.In(loc), msg.Seed, clientKey); err != nil {
		return err
	}

	// Verify session identifier
	if !bytes.Equal(rh.sid, msg.SID) {
		log.Lvlf1("handleI1: %v %v", rh.sid, msg.SID)
		return errorWrongSession
	}

	// Setup CoSi instance
	rh.cosi = cosi.NewCosi(rh.Suite(), rh.Private(), rh.Roster().Publics())

	// Compute hash of the client's message
	hi1, err := hashMessage(rh.Suite(), msg)
	if err != nil {
		return err
	}

	// Compute encrypted PVSS shares for group members
	grp := rh.groupNum[idx]
	groupKeys := rh.serverKeys[grp]
	t := int(rh.thresholds[grp])
	secret := rh.Suite().Scalar().Pick(random.Stream)
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))
	encShares, pubPoly, err := pvss.EncShares(rh.Suite(), H, groupKeys, secret, t)
	if err != nil {
		return err
	}

	// Wrap encrypted shares to keep track of source and target servers
	shares := make([]*Share, len(encShares))
	for i, share := range encShares {
		shares[i] = &Share{
			Source:      rh.TreeNode().RosterIndex,
			Target:      rh.servers[grp][i].RosterIndex,
			PubVerShare: share,
		}
	}

	// Setup R1 message
	_, coeffs := pubPoly.Info()
	r1 := &R1{
		SID:       rh.sid,
		HI1:       hi1,
		EncShares: shares,
		Coeffs:    coeffs,
		V:         rh.cosi.CreateCommitment(random.Stream),
	}

	// Sign R1 message
	if err := signSchnorr(rh.Suite(), rh.Private(), r1); err != nil {
		return err
	}

	// Send R1 message
	return rh.SendTo(rh.Root(), r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {
	msg := &r1.R1
	src := r1.RosterIndex
	grp := rh.groupNum[src]
	pos := rh.groupPos[src]
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Verify R1 message signature
	if err := verifySchnorr(rh.Suite(), rh.serverKeys[grp][pos], msg); err != nil {
		return err
	}

	// Verify session identifier
	if !bytes.Equal(rh.sid, msg.SID) {
		return errorWrongSession
	}

	// Verify that the server replied to the correct I1 message
	hi1, err := hashMessage(rh.Suite(), rh.i1)
	if err != nil {
		return err
	}
	if !bytes.Equal(hi1, msg.HI1) {
		return errors.New("server replied to wrong I1 message")
	}

	// Record server commit
	rh.commits[src] = msg.V

	// Return, if we already committed to secrets before
	if len(rh.chosenSecrets) > 0 {
		return nil
	}

	// Verify encrypted shares and record valid ones
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.sid))
	pubPoly := share.NewPubPoly(rh.Suite(), H, msg.Coeffs)
	for _, encShare := range msg.EncShares {
		pos := encShare.PubVerShare.S.I
		sH := pubPoly.Eval(pos).V
		key := rh.serverKeys[grp][pos]
		if pvss.VerifyEncShare(rh.Suite(), H, key, sH, encShare.PubVerShare) == nil {
			src := encShare.Source
			tgt := encShare.Target
			if _, ok := rh.records[src]; !ok {
				rh.records[src] = make(map[int]*Record)
			}
			rh.records[src][tgt] = &Record{
				Eval:     sH,
				EncShare: encShare.PubVerShare,
				DecShare: nil,
			}
		}
	}

	// Check if there is at least a threshold number of reconstructable secrets
	// in each group. If yes proceed to the next phase. Note the double-usage
	// of the threshold which is used to determine if enough valid shares for a
	// single secret are available and if enough secrets for a given group are
	// available
	goodSecrets := make(map[int][]int)
	for i, servers := range rh.servers {
		var secret []int
		for _, server := range servers {
			src := server.RosterIndex
			if shares, ok := rh.records[src]; ok && int(rh.thresholds[i]) <= len(shares) {
				secret = append(secret, src)
			}
		}
		if int(rh.thresholds[i]) <= len(secret) {
			goodSecrets[i] = secret
		}
	}

	// Proceed, if there are enough good secrets and more than 2/3 of servers replied
	// TODO: maybe we want to have a timer here to give nodes chances to send their replies
	if len(goodSecrets) == rh.groups && 2*rh.nodes/3 < len(rh.records) {

		for i := range rh.servers {
			// Randomly remove some secrets so that a threshold of secrets remain
			rand := random.Bytes(rh.Suite().Hash().Size(), random.Stream)
			prng := rh.Suite().Cipher(rand)
			secrets := goodSecrets[i]
			l := len(secrets) - int(rh.thresholds[i])
			for j := 0; j < l; j++ {
				k := int(random.Uint32(prng) % uint32(len(secrets)))
				delete(rh.records, secrets[k]) // delete not required records
			}
		}

		// Recover chosen secrets from records
		rh.chosenSecrets = chosenSecrets(rh.records)

		// Clear CoSi mask
		for i := 0; i < rh.nodes; i++ {
			rh.cosi.SetMaskBit(i, false)
		}

		// Set our own masking bit
		rh.cosi.SetMaskBit(rh.TreeNode().RosterIndex, true)

		// Collect commits and mark participating nodes
		rh.participants = make([]int, 0)
		var subComms []abstract.Point
		for i, V := range rh.commits {
			subComms = append(subComms, V)
			rh.cosi.SetMaskBit(i, true)
			rh.participants = append(rh.participants, i)
		}

		// Compute aggregate commit
		rh.cosi.Commit(random.Stream, subComms)

		// Compute message: statement = SID || chosen secrets
		buf := new(bytes.Buffer)
		if _, err := buf.Write(rh.sid); err != nil {
			return err
		}
		for _, cs := range rh.chosenSecrets {
			binary.Write(buf, binary.LittleEndian, cs)
		}
		rh.statement = buf.Bytes()

		// Compute CoSi challenge
		if _, err := rh.cosi.CreateChallenge(rh.statement); err != nil {
			return err
		}

		// Prepare a message for each server of a group and send it
		for i, servers := range rh.servers {
			for _, server := range servers {
				// Among the good secrets chosen previously collect all valid
				// shares, proofs, and polynomial commits intended for the
				// target server
				var encShares []*Share
				var evals []abstract.Point
				tgt := server.RosterIndex
				for _, src := range rh.indices[i] {
					if record, ok := rh.records[src][tgt]; ok {
						encShare := &Share{
							Source:      src,
							Target:      tgt,
							PubVerShare: record.EncShare,
						}
						encShares = append(encShares, encShare)
						evals = append(evals, record.Eval)
					}
				}
				i2 := &I2{
					Sig:           []byte{0},
					SID:           rh.sid,
					ChosenSecrets: rh.chosenSecrets,
					EncShares:     encShares,
					Evals:         evals,
					C:             rh.cosi.GetChallenge(),
				}
				if err := signSchnorr(rh.Suite(), rh.Private(), i2); err != nil {
					return err
				}
				rh.i2s[tgt] = i2
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

	// Verify I2 message signature
	if err := verifySchnorr(rh.Suite(), rh.clientKey, msg); err != nil {
		return err
	}

	// Verify session identifier
	if !bytes.Equal(rh.sid, msg.SID) {
		return errorWrongSession
	}

	// Verify encrypted shares and record valid ones
	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(rh.sid))
	rh.records = make(map[int]map[int]*Record)
	for i, encShare := range msg.EncShares {
		pos := encShare.PubVerShare.S.I
		src := encShare.Source
		tgt := encShare.Target
		grp := rh.groupNum[src]
		key := rh.serverKeys[grp][pos]
		if _, ok := rh.records[src]; !ok {
			rh.records[src] = make(map[int]*Record)
		}
		if pvss.VerifyEncShare(rh.Suite(), H, key, msg.Evals[i], encShare.PubVerShare) == nil {
			rh.records[src][tgt] = &Record{
				Eval:     msg.Evals[i],
				EncShare: encShare.PubVerShare,
				DecShare: nil,
			}
		}
	}

	// Compute hash of the client's message
	hi2, err := hashMessage(rh.Suite(), msg)
	if err != nil {
		return err
	}

	// Record chosen secrets
	rh.chosenSecrets = msg.ChosenSecrets

	// Check that chosen secrets satisfy thresholds
	counter := make([]int, rh.groups)
	for _, src := range rh.chosenSecrets {
		i := rh.groupNum[int(src)]
		counter[i]++
	}
	for i, t := range counter {
		if t < int(rh.thresholds[i]) {
			return fmt.Errorf("not enough chosen secrets for group %v", i)
		}
	}

	rh.cosi.Challenge(msg.C)
	r, err := rh.cosi.CreateResponse()
	if err != nil {
		return err
	}

	// Setup R2 message
	r2 := &R2{
		SID: rh.sid,
		HI2: hi2,
		R:   r,
	}

	// Sign R2 message
	if err := signSchnorr(rh.Suite(), rh.Private(), r2); err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {
	msg := &r2.R2
	src := r2.RosterIndex
	grp := rh.groupNum[src]
	pos := rh.groupPos[src]
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Verify R2 message signature
	if err := verifySchnorr(rh.Suite(), rh.serverKeys[grp][pos], msg); err != nil {
		return err
	}

	// Verify session identifier
	if !bytes.Equal(rh.sid, msg.SID) {
		return errorWrongSession
	}

	// Verify that server replied to the correct I2 message
	hi2, err := hashMessage(rh.Suite(), rh.i2s[src])
	if err != nil {
		return err
	}
	if !bytes.Equal(hi2, msg.HI2) {
		return errors.New("server replied to wrong I2 message")
	}

	// Record R2 message
	rh.r2s[src] = msg

	// TODO: What condition to proceed? We need at least a reply from the 2/3 of
	// nodes that we chose earlier. Should we have a timer?
	if len(rh.r2s) == rh.nodes-1 {
		var responses []abstract.Scalar
		for _, src := range rh.participants {
			responses = append(responses, rh.r2s[src].R)
		}
		if _, err := rh.cosi.Response(responses); err != nil {
			return err
		}
		rh.cosig = rh.cosi.Signature()
		if err := cosi.VerifySignature(rh.Suite(), rh.Roster().Publics(), rh.statement, rh.cosig); err != nil {
			return err
		}
		rh.i3 = &I3{
			SID:   rh.sid,
			CoSig: rh.cosig,
		}
		if err := signSchnorr(rh.Suite(), rh.Private(), rh.i3); err != nil {
			return err
		}
		if err := rh.Broadcast(rh.i3); err != nil {
			return err
		}
	}
	return nil
}

func (rh *RandHound) handleI3(i3 WI3) error {
	msg := &i3.I3

	// Verify I3 message signature
	if err := verifySchnorr(rh.Suite(), rh.clientKey, msg); err != nil {
		return err
	}

	// Verify session identifier
	if !bytes.Equal(rh.sid, msg.SID) {
		return errorWrongSession
	}

	// Compute message: statement = SID || chosen secrets
	buf := new(bytes.Buffer)
	if _, err := buf.Write(rh.sid); err != nil {
		return err
	}
	for _, cs := range rh.chosenSecrets {
		binary.Write(buf, binary.LittleEndian, cs)
	}
	rh.statement = buf.Bytes()

	// Verify collective signature (TODO: check that more than 2/3 of participants have signed)
	if err := cosi.VerifySignature(rh.Suite(), rh.Roster().Publics(), rh.statement, msg.CoSig); err != nil {
		return err
	}

	// Compute hash of the client's message
	hi3, err := hashMessage(rh.Suite(), msg)
	if err != nil {
		return err
	}

	H, _ := rh.Suite().Point().Pick(nil, rh.Suite().Cipher(msg.SID))
	var decShares []*Share

	for src, records := range rh.records {
		for tgt, record := range records {
			decShare, err := pvss.DecShare(rh.Suite(), H, rh.Public(), record.Eval, rh.Private(), record.EncShare)
			if err == nil {
				s := &Share{
					Source:      src,
					Target:      tgt,
					PubVerShare: decShare,
				}
				decShares = append(decShares, s)
			}
		}
	}

	r3 := &R3{
		SID:       rh.sid,
		HI3:       hi3,
		DecShares: decShares,
	}

	if err := signSchnorr(rh.Suite(), rh.Private(), r3); err != nil {
		return err
	}

	return rh.SendTo(rh.Root(), r3)
}

func (rh *RandHound) handleR3(r3 WR3) error {
	msg := &r3.R3
	src := r3.RosterIndex
	grp := rh.groupNum[src]
	pos := rh.groupPos[src]
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// If the collective secret is already available, ignore all further incoming messages
	if rh.SecretReady {
		return nil
	}

	// Verify R3 message signature
	if err := verifySchnorr(rh.Suite(), rh.serverKeys[grp][pos], msg); err != nil {
		return err
	}

	// Verify that server replied to the correct I3 message
	hi3, err := hashMessage(rh.Suite(), rh.i3)
	if err != nil {
		return err
	}
	if !bytes.Equal(hi3, msg.HI3) {
		return errors.New("server replied to wrong I3 message")
	}

	// Record R3 message
	rh.r3s[src] = msg

	// Verify decrypted shares and record valid ones
	G := rh.Suite().Point().Base()
	keys := rh.Roster().Publics()
	for _, share := range msg.DecShares {
		src := share.Source
		tgt := share.Target
		if _, ok := rh.records[src][tgt]; !ok {
			continue
		}
		record := rh.records[src][tgt]
		key := keys[tgt]
		encShare := record.EncShare
		decShare := share.PubVerShare
		if pvss.VerifyDecShare(rh.Suite(), G, key, encShare, decShare) == nil {
			record.DecShare = decShare
			rh.records[src][tgt] = record
		}
	}

	proceed := true
	for src, records := range rh.records {
		c := 0 // enough shares?
		for _, record := range records {
			if record.EncShare != nil && record.DecShare != nil {
				c++
			}
		}
		grp := rh.groupNum[src]
		if c < int(rh.thresholds[grp]) {
			proceed = false
		}
	}

	if len(rh.r3s) == rh.nodes-1 && !proceed {
		rh.Done <- true
		return errors.New("some chosen secrets are not reconstructable")
	}

	if proceed && !rh.SecretReady {
		rh.SecretReady = true
		rh.Done <- true
	}
	return nil
}

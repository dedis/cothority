package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/share/pvss"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/kyber/util/random"
)

// Some error definitions.
var errorWrongSession = errors.New("wrong session identifier")

func (rh *RandHound) handleI1(i1 WI1) error {
	// Executed on the randomness servers
	msg := &i1.I1
	var err error
	src := i1.RosterIndex
	idx := rh.TreeNode().RosterIndex
	keys := rh.Roster().Publics()
	nodes := len(keys)
	clientKey := keys[src]

	// Verify I1 message signature
	if err = verifySchnorr(rh.Suite(), clientKey, msg); err != nil {
		return err
	}

	// Setup session
	if rh.Session, err = rh.newSession(nodes, msg.Groups, msg.Purpose, msg.Time, msg.Seed, clientKey); err != nil {
		return err
	}

	// Verify session identifier
	if !bytes.Equal(rh.sid, msg.SID) {
		return errorWrongSession
	}

	// Compute hash of the client's message
	hi1, err := hashMessage(rh.Suite(), msg)
	if err != nil {
		return err
	}

	// Compute encrypted PVSS shares for group members
	grp := rh.groupNum[idx]
	groupKeys := rh.serverKeys[grp]
	t := int(rh.thresholds[grp])
	secret := rh.Suite().Scalar().Pick(random.New())
	H := rh.Suite().Point().Pick(rh.Suite().XOF(msg.SID))
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
	rh.cosi.v, rh.cosi.V = cosi.Commit(rh.Suite())
	_, coeffs := pubPoly.Info()
	r1 := &R1{
		SID:       rh.sid,
		HI1:       hi1,
		EncShares: shares,
		Coeffs:    coeffs,
		V:         rh.cosi.V,
	}

	// Sign R1 message
	if err := signSchnorr(rh.Suite(), rh.Private(), r1); err != nil {
		return err
	}

	// Send R1 message
	return rh.SendTo(rh.Root(), r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {
	// Executed on the client
	msg := &r1.R1
	src := r1.RosterIndex
	grp := rh.groupNum[src]
	pos := rh.groupPos[src]
	rh.mutex.Lock()
	defer rh.mutex.Unlock()

	// Do not accept new R1 messages if we have already committed to secrets before
	if len(rh.chosenSecrets) > 0 {
		return nil
	}

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

	// Verify encrypted shares and record valid ones
	H := rh.Suite().Point().Pick(rh.Suite().XOF(rh.sid))
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
	// (TODO: maybe we want to have a timer here to give nodes chances to send their replies)
	if len(goodSecrets) == rh.groups && 2*rh.nodes/3 < len(rh.records) {

		for i := range rh.servers {
			// Randomly remove some secrets so that a threshold of secrets remain
			secrets := goodSecrets[i]
			l := len(secrets) - int(rh.thresholds[i])
			toDelete := rand.Perm(len(secrets))[0:l]
			for _, j := range toDelete {
				delete(rh.records, secrets[j]) // delete not required records
			}
		}

		// Recover chosen secrets from records
		rh.chosenSecrets = chosenSecrets(rh.records)

		// Clear CoSi mask
		rh.cosi.mask, err = cosi.NewMask(rh.Suite(), rh.Roster().Publics(), rh.Public())
		if err != nil {
			return err
		}

		// Set our own masking bit
		rh.cosi.mask.SetBit(rh.TreeNode().RosterIndex, true)

		// Collect commits and mark participating nodes
		var subComms []kyber.Point
		var masks [][]byte
		for i, V := range rh.commits {
			subComms = append(subComms, V)
			rh.cosi.mask.SetBit(i, true)
			masks = append(masks, rh.cosi.mask.Mask())
		}

		// Compute aggregate commit
		cosi.AggregateCommitments(rh.Suite(), subComms, masks)

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
		if rh.cosi.c, err = cosi.Challenge(rh.Suite(), rh.cosi.V, rh.Public(), rh.statement); err != nil {
			return err
		}

		// Prepare a message for each server of a group and send it
		for i, servers := range rh.servers {
			for _, server := range servers {
				// Among the good secrets chosen previously collect all valid
				// shares, proofs, and polynomial commits intended for the
				// target server
				var encShares []*Share
				var evals []kyber.Point
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
					C:             rh.cosi.c,
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
	// Executed on the randomness servers
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
	H := rh.Suite().Point().Pick(rh.Suite().XOF(rh.sid))
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

	rh.cosi.c = msg.C
	// BUG: this is not correct
	r, err := cosi.Response(rh.Suite(), rh.Private(), rh.cosi.v, msg.C)
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
	// Executed on the client
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
	// NOTE: we only consider messages from servers that committed earlier
	if _, ok := rh.commits[src]; ok {
		rh.r2s[src] = msg
	}

	// Proceed once we have all responses from servers that committed earlier
	if len(rh.commits) <= len(rh.r2s) {

		var responses []kyber.Scalar
		for src := range rh.commits {
			responses = append(responses, rh.r2s[src].R)
		}

		if rh.cosi.r, err = cosi.AggregateResponses(rh.Suite(), responses); err != nil {
			return err
		}
		rh.cosig, err = cosi.Sign(rh.Suite(), rh.cosi.V, rh.cosi.r, rh.cosi.mask)
		if err != nil {
			return err
		}
		if err := cosi.Verify(rh.Suite(), rh.Roster().Publics(), rh.statement, rh.cosig, cosi.CompletePolicy{}); err != nil {
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
			return err[0]
		}
	}
	return nil
}

func (rh *RandHound) handleI3(i3 WI3) error {
	// Executed on the randomness servers
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
	if err := cosi.Verify(rh.Suite(), rh.Roster().Publics(), rh.statement, msg.CoSig, cosi.CompletePolicy{}); err != nil {
		return err
	}

	// Compute hash of the client's message
	hi3, err := hashMessage(rh.Suite(), msg)
	if err != nil {
		return err
	}

	H := rh.Suite().Point().Pick(rh.Suite().XOF(msg.SID))
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
	// Executed on the client
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

package randhound

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
)

func (rh *RandHound) handleI1(i1 WI1) error {

	// If we are not a leaf, forward i1 to children
	if !rh.Node.IsLeaf() {
		if err := rh.sendToChildren(&i1.I1); err != nil {
			return err
		}
	}
	rh.Peer.i1 = &i1.I1

	// Store group parameters
	rh.GID = i1.I1.GID
	rh.Group = i1.I1.Group

	// Store session parameters
	rh.SID = i1.I1.SID
	rh.Session = i1.I1.Session

	// Choose peer's trustee-selsection randomness
	hs := rh.Node.Suite().Hash().Size()
	rs := make([]byte, hs)
	random.Stream.XORKeyStream(rs, rs)
	rh.Peer.Rs = rs

	rh.Peer.r1 = &R1{
		Src: rh.nodeIdx(),
		HI1: rh.hash(
			rh.SID,
			rh.GID,
			rh.Peer.i1.HRc,
		),
		HRs: rh.hash(rh.Peer.Rs),
	}
	return rh.SendTo(rh.Parent(), rh.Peer.r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {

	// If we are not the root, forward r1 to parent
	if !rh.Node.IsRoot() {
		if err := rh.SendTo(rh.Parent(), &r1.R1); err != nil {
			return err
		}
	} else {

		// Verify reply
		if !bytes.Equal(r1.R1.HI1, rh.hash(rh.SID, rh.GID, rh.Leader.i1.HRc)) {
			return errors.New(fmt.Sprintf("R1: peer %d replied to wrong I1 message", r1.Src))
		}

		// Collect replies of the peers
		rh.Leader.r1[r1.Src] = &r1.R1

		// Continue, once all replies have arrived
		if len(rh.Leader.r1) == rh.Group.N-1 {
			rh.Leader.i2 = &I2{
				SID: rh.SID,
				Rc:  rh.Leader.Rc,
			}
			if err := rh.sendToChildren(rh.Leader.i2); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rh *RandHound) handleI2(i2 WI2) error {

	// If we are not a leaf, forward i2 to children
	if !rh.Node.IsLeaf() {
		if err := rh.sendToChildren(&i2.I2); err != nil {
			return err
		}
	}

	// Verify session ID
	if !bytes.Equal(rh.SID, i2.I2.SID) {
		return errors.New(fmt.Sprintf("I2: peer %d received message with incorrect session ID", rh.nodeIdx()))
	}

	rh.Peer.i2 = &i2.I2

	// Construct deal
	longPair := &config.KeyPair{
		rh.Node.Suite(),
		rh.Node.Public(),
		rh.Node.Private(),
	}
	secretPair := config.NewKeyPair(rh.Node.Suite())
	_, trustees := rh.chooseTrustees(rh.Peer.i2.Rc, rh.Peer.Rs)
	deal := &poly.Deal{}
	deal.ConstructDeal(secretPair, longPair, rh.Group.T, rh.Group.R, trustees)
	db, err := deal.MarshalBinary()
	if err != nil {
		return err
	}

	rh.Peer.r2 = &R2{
		Src: rh.nodeIdx(),
		HI2: rh.hash(
			rh.SID,
			rh.Peer.i2.Rc),
		Rs:   rh.Peer.Rs,
		Deal: db,
	}
	return rh.SendTo(rh.Parent(), rh.Peer.r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {

	// If we are not the root, forward r2 to parent
	if !rh.Node.IsRoot() {
		if err := rh.SendTo(rh.Parent(), &r2.R2); err != nil {
			return err
		}
	} else {

		// Verify reply
		if !bytes.Equal(r2.R2.HI2, rh.hash(rh.SID, rh.Leader.i2.Rc)) {
			return errors.New(fmt.Sprintf("R2: peer %d replied to wrong I2 message", r2.Src))
		}

		// Collect replies of the peers
		rh.Leader.r2[r2.Src] = &r2.R2

		// Initialise deal
		deal := &poly.Deal{}
		deal.UnmarshalInit(rh.Group.T, rh.Group.R, rh.Group.K, rh.Node.Suite())
		if err := deal.UnmarshalBinary(r2.Deal); err != nil {
			return err
		}

		// Initialise state with deal
		state := poly.State{}
		ps := state.Init(*deal)
		rh.Leader.states[r2.Src] = ps

		// Continue, once all replies have arrived
		if len(rh.Leader.r2) == rh.Group.N-1 {
			rh.Leader.i3 = &I3{
				SID: rh.SID,
				R2s: rh.Leader.r2,
			}
			return rh.sendToChildren(rh.Leader.i3)
		}
	}
	return nil
}

func (rh *RandHound) handleI3(i3 WI3) error {

	// If we are not a leaf, forward i3 to children
	if !rh.Node.IsLeaf() {
		if err := rh.sendToChildren(&i3.I3); err != nil {
			return err
		}
	}

	// Verify session ID
	if !bytes.Equal(rh.SID, i3.I3.SID) {
		return errors.New(fmt.Sprintf("I3: peer %d received message with incorrect session ID", rh.nodeIdx()))
	}

	rh.Peer.i3 = &i3.I3

	longPair := &config.KeyPair{
		rh.Node.Suite(),
		rh.Node.Public(),
		rh.Node.Private(),
	}

	r3resps := []R3Resp{}
	r4shares := []R4Share{}
	for src, r2 := range rh.Peer.i3.R2s {

		if !bytes.Equal(r2.HI2, rh.Peer.r2.HI2) {
			return errors.New("I3: R2 contains wrong I2 hash")
		}

		// Unmarshal Deal
		deal := &poly.Deal{}
		deal.UnmarshalInit(rh.Group.T, rh.Group.R, rh.Group.K, rh.Node.Suite())
		if err := deal.UnmarshalBinary(r2.Deal); err != nil {
			return err
		}

		// Determine other peers who chose the current peer as a trustee
		shareIdx, _ := rh.chooseTrustees(rh.Peer.i2.Rc, r2.Rs)
		if k, ok := shareIdx[rh.nodeIdx()]; ok { // k is the share index we received from the peer 'src'
			resp, err := deal.ProduceResponse(k, longPair)
			if err != nil {
				return err
			}

			rb, err := resp.MarshalBinary()
			if err != nil {
				return err
			}
			r3resps = append(r3resps, R3Resp{src, k, rb})

			share := deal.RevealShare(k, longPair)
			r4shares = append(r4shares, R4Share{src, k, share})
		}
	}
	rh.Peer.shares = r4shares // save revealed shares for later

	rh.Peer.r3 = &R3{
		Src: rh.nodeIdx(),
		HI3: rh.hash(
			rh.SID,
			rh.Peer.r2.HI2), // TODO: is this enough?
		Responses: r3resps,
	}
	return rh.SendTo(rh.Parent(), rh.Peer.r3)
}

func (rh *RandHound) handleR3(r3 WR3) error {

	// If we are not the root, forward r3 to parent
	if !rh.Node.IsRoot() {
		if err := rh.SendTo(rh.Parent(), &r3.R3); err != nil {
			return err
		}
	} else {

		// Verify reply
		if !bytes.Equal(r3.R3.HI3, rh.hash(rh.SID, rh.hash(rh.SID, rh.Leader.i2.Rc))) {
			return errors.New(fmt.Sprintf("R3: peer %d replied to wrong I3 message", r3.Src))
		}

		// Collect replies of the peers
		rh.Leader.r3[r3.Src] = &r3.R3

		for _, r3resp := range rh.Leader.r3[r3.Src].Responses {

			resp := &poly.Response{}
			resp.UnmarshalInit(rh.Node.Suite())
			if err := resp.UnmarshalBinary(r3resp.Resp); err != nil {
				return err
			}

			// Verify that response is securely bound to promise
			if err := rh.Leader.states[r3resp.Dealer].AddResponse(r3resp.ShareIdx, resp); err != nil {
				return err
			}
		}

		// Continue, once all replies have arrived
		if len(rh.Leader.r3) == rh.Group.N-1 {
			rh.Leader.i4 = &I4{
				SID: rh.SID,
				R2s: rh.Leader.r2,
			}
			return rh.sendToChildren(rh.Leader.i4)
		}
	}
	return nil
}

func (rh *RandHound) handleI4(i4 WI4) error {

	// If we are not a leaf, forward i4 to children
	if !rh.Node.IsLeaf() {
		if err := rh.sendToChildren(&i4.I4); err != nil {
			return err
		}
	}

	// Verify session ID
	if !bytes.Equal(rh.SID, i4.I4.SID) {
		return errors.New(fmt.Sprintf("I4: peer %d received message with incorrect session ID", rh.nodeIdx()))
	}

	rh.Peer.i4 = &i4.I4

	rh.Peer.r4 = &R4{
		Src: rh.nodeIdx(),
		HI4: rh.hash(
			rh.SID,
			make([]byte, 0)), // TODO: unpack R2s, see I4
		Shares: rh.Peer.shares,
	}
	return rh.SendTo(rh.Parent(), rh.Peer.r4)
}

func (rh *RandHound) handleR4(r4 WR4) error {

	// If we are not the root, forward r4 to parent
	if !rh.Node.IsRoot() {
		if err := rh.SendTo(rh.Parent(), &r4.R4); err != nil {
			return err
		}
	} else {

		// Verify reply
		if !bytes.Equal(r4.R4.HI4, rh.hash(rh.SID, make([]byte, 0))) {
			return errors.New(fmt.Sprintf("R4: peer %d replied to wrong I4 message", r4.Src))
		}

		// Collect replies of the peers
		rh.Leader.r4[r4.Src] = &r4.R4

		// Continue, once all replies have arrived
		if len(rh.Leader.r4) == rh.Group.N-1 {
			// Process shares of i-th peer
			for i, _ := range rh.Leader.r4 {
				for _, r4share := range rh.Leader.r4[i].Shares {
					j := r4share.Dealer
					idx := r4share.Idx
					share := r4share.Share

					shareIdx, _ := rh.chooseTrustees(rh.Leader.Rc, rh.Leader.r2[j].Rs)
					if idx != shareIdx[i] {
						return errors.New(fmt.Sprintf("R4: server %d claimed share it wasn't dealt", i))
					}

					// Verify share
					if err := rh.Leader.states[j].Deal.VerifyRevealedShare(idx, share); err != nil {
						return err
					}

					// Store share
					rh.Leader.states[j].PriShares.SetShare(idx, share)
				}
			}

			// Generate the output
			output := rh.Node.Suite().Secret().Zero()
			for _, state := range rh.Leader.states {
				output.Add(output, state.PriShares.Secret())
			}

			rb, err := output.MarshalBinary()
			if err != nil {
				return err
			}

			rh.Leader.Done <- true
			rh.Leader.Result <- rb
		}
	}
	return nil
}

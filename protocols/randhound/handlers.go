// Implementation of the four RandHound phases.

package randhound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
)

// I1 is the message sent by the leader to the peers in phase 1.
type I1 struct {
	SID     []byte   // Session identifier: hash of session info block
	Session *Session // Session parameters
	GID     []byte   // Group identifier: hash of group parameter block
	Group   *Group   // Group parameters
	HRc     []byte   // Client's trustee-randomness commit
}

// R1 is the reply sent by the peers to the leader in phase 1.
type R1 struct {
	Src uint32 // Source of the message
	HI1 []byte // Hash of I1 message
	HRs []byte // Peer's trustee-randomness commit
}

// I2 is the message sent by the leader to the peers in phase 2.
type I2 struct {
	SID []byte // Session identifier
	Rc  []byte // Leader's trustee-selection randomnes
}

// R2 is the reply sent by the peers to the leader in phase 2.
type R2 struct {
	Src  uint32 // Source of the message
	HI2  []byte // Hash of I2 message
	Rs   []byte // Peers' trustee-selection randomness
	Deal []byte // Peer's secret-sharing to trustees
}

// I3 is the message sent by the leader to the peers in phase 3.
type I3 struct {
	SID []byte         // Session identifier
	R2s map[uint32]*R2 // Leaders's list of signed R2 messages; empty slices represent missing R2 messages
}

// R3 is the reply sent by the peers to the leader in phase 3.
type R3 struct {
	Src       uint32   // Source of the message
	HI3       []byte   // Hash of I3 message
	Responses []R3Resp // Responses to dealt secret-shares
}

// R3Resp encapsulates a peer's response together with some metadata.
type R3Resp struct {
	DealerIdx uint32 // Dealer's index in the peer list
	ShareIdx  uint32 // Share's index in deal we are validating
	Resp      []byte // Encoded response to dealer's deal
}

// I4 is the message sent by the leader to the peers in phase 4.
type I4 struct {
	SID     []byte               // Session identifier
	Invalid map[uint32]*[]uint32 // Map to mark invalid responses
}

// R4 is the reply sent by the peers to the leader in phase 4.
type R4 struct {
	Src    uint32              // Source of the message
	HI4    []byte              // Hash of I4 message
	Shares map[uint32]*R4Share // Revealed secret-shares
}

// R4Share encapsulates a peer's share together with some metadata.
type R4Share struct {
	DealerIdx uint32          // Dealer's index in the peer list
	ShareIdx  uint32          // Share's index in dealer's deal
	Share     abstract.Secret // Decrypted share dealt to this server
}

// WI1 is a SDA-wrapper around I1
type WI1 struct {
	*sda.TreeNode
	I1
}

// WI2 is a SDA-wrapper around I2
type WI2 struct {
	*sda.TreeNode
	I2
}

// WI3 is a SDA-wrapper around I3
type WI3 struct {
	*sda.TreeNode
	I3
}

// WI4 is a SDA-wrapper around I4
type WI4 struct {
	*sda.TreeNode
	I4
}

// WR1 is a SDA-wrapper around R1
type WR1 struct {
	*sda.TreeNode
	R1
}

// WR2 is a SDA-wrapper around R2
type WR2 struct {
	*sda.TreeNode
	R2
}

// WR3 is a SDA-wrapper around R3
type WR3 struct {
	*sda.TreeNode
	R3
}

// WR4 is a SDA-wrapper around R4
type WR4 struct {
	*sda.TreeNode
	R4
}

func (rh *RandHound) handleI1(i1 WI1) error {

	// If we are not a leaf, forward i1 to children
	if !rh.IsLeaf() {
		if err := rh.SendToChildren(&i1.I1); err != nil {
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
	hs := rh.Suite().Hash().Size()
	rs := make([]byte, hs)
	random.Stream.XORKeyStream(rs, rs)
	rh.Peer.rs = rs

	rh.Peer.r1 = &R1{
		Src: rh.index(),
		HI1: rh.hash(
			rh.SID,
			rh.GID,
			rh.Peer.i1.HRc,
		),
		HRs: rh.hash(rh.Peer.rs),
	}
	return rh.SendToParent(rh.Peer.r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {

	// If we are not the root, forward r1 to parent
	if !rh.IsRoot() {
		if err := rh.SendToParent(&r1.R1); err != nil {
			return err
		}
	} else {

		// Verify reply
		if !bytes.Equal(r1.R1.HI1, rh.hash(rh.SID, rh.GID, rh.Leader.i1.HRc)) {
			return fmt.Errorf("R1: peer %d replied to wrong I1 message", r1.Src)
		}

		// Collect replies of the peers
		rh.Leader.r1[r1.Src] = &r1.R1

		if uint32(len(rh.Leader.r1)) == rh.Group.N-1 {
			// Continue, once all replies have arrived

			rh.Leader.i2 = &I2{
				SID: rh.SID,
				Rc:  rh.Leader.rc,
			}
			if err := rh.SendToChildren(rh.Leader.i2); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rh *RandHound) handleI2(i2 WI2) error {

	// If we are not a leaf, forward i2 to children
	if !rh.IsLeaf() {
		if err := rh.SendToChildren(&i2.I2); err != nil {
			return err
		}
	}

	// Verify session ID
	if !bytes.Equal(rh.SID, i2.I2.SID) {
		return fmt.Errorf("I2: peer %d received message with incorrect session ID", rh.index())
	}

	rh.Peer.i2 = &i2.I2

	// Construct deal
	longPair := &config.KeyPair{
		rh.Suite(),
		rh.Public(),
		rh.Private(),
	}
	secretPair := config.NewKeyPair(rh.Suite())
	_, trustees := rh.chooseTrustees(rh.Peer.i2.Rc, rh.Peer.rs)
	deal := &poly.Deal{}
	deal.ConstructDeal(secretPair, longPair, int(rh.Group.T), int(rh.Group.R), trustees)
	db, err := deal.MarshalBinary()
	if err != nil {
		return err
	}

	rh.Peer.r2 = &R2{
		Src: rh.index(),
		HI2: rh.hash(
			rh.SID,
			rh.Peer.i2.Rc),
		Rs:   rh.Peer.rs,
		Deal: db,
	}
	return rh.SendToParent(rh.Peer.r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {

	// If we are not the root, forward r2 to parent
	if !rh.IsRoot() {
		if err := rh.SendToParent(&r2.R2); err != nil {
			return err
		}
	} else {

		// Verify reply
		if !bytes.Equal(r2.R2.HI2, rh.hash(rh.SID, rh.Leader.i2.Rc)) {
			return fmt.Errorf("R2: peer %d replied to wrong I2 message", r2.Src)
		}

		// Collect replies of the peers
		rh.Leader.r2[r2.Src] = &r2.R2

		// Initialise deal
		deal := &poly.Deal{}
		deal.UnmarshalInit(int(rh.Group.T), int(rh.Group.R), int(rh.Group.K), rh.Suite())
		if err := deal.UnmarshalBinary(r2.Deal); err != nil {
			return err
		}

		// Initialise state with deal
		state := poly.State{}
		ps := state.Init(*deal)
		rh.Leader.states[r2.Src] = ps

		if uint32(len(rh.Leader.r2)) == rh.Group.N-1 {
			// Continue, once all replies have arrived

			rh.Leader.i3 = &I3{
				SID: rh.SID,
				R2s: rh.Leader.r2,
			}
			return rh.SendToChildren(rh.Leader.i3)
		}
	}
	return nil
}

func (rh *RandHound) handleI3(i3 WI3) error {

	// If we are not a leaf, forward i3 to children
	if !rh.IsLeaf() {
		if err := rh.SendToChildren(&i3.I3); err != nil {
			return err
		}
	}

	// Verify session ID
	if !bytes.Equal(rh.SID, i3.I3.SID) {
		return fmt.Errorf("I3: peer %d received message with incorrect session ID", rh.index())
	}

	rh.Peer.i3 = &i3.I3

	longPair := &config.KeyPair{
		rh.Suite(),
		rh.Public(),
		rh.Private(),
	}

	responses := []R3Resp{}
	for i, r2 := range rh.Peer.i3.R2s {

		if !bytes.Equal(r2.HI2, rh.Peer.r2.HI2) {
			return errors.New("I3: R2 contains wrong I2 hash")
		}

		// Unmarshal Deal
		deal := &poly.Deal{}
		deal.UnmarshalInit(int(rh.Group.T), int(rh.Group.R), int(rh.Group.K), rh.Suite())
		if err := deal.UnmarshalBinary(r2.Deal); err != nil {
			return err
		}

		// Determine other peers who chose the current peer as a trustee
		shareIdx, _ := rh.chooseTrustees(rh.Peer.i2.Rc, r2.Rs)
		if j, ok := shareIdx[rh.index()]; ok { // j is the share index we received from the ith peer
			resp, err := deal.ProduceResponse(int(j), longPair)
			if err != nil {
				return err
			}

			rb, err := resp.MarshalBinary()
			if err != nil {
				return err
			}

			responses = append(responses, R3Resp{DealerIdx: i, ShareIdx: j, Resp: rb})

			share := deal.RevealShare(int(j), longPair)
			rh.Peer.shares[i] = &R4Share{DealerIdx: i, ShareIdx: j, Share: share}
		}
	}

	rh.Peer.r3 = &R3{
		Src: rh.index(),
		HI3: rh.hash(
			rh.SID,
			rh.Peer.r2.HI2), // TODO: is this enough?
		Responses: responses,
	}
	return rh.SendToParent(rh.Peer.r3)
}

func (rh *RandHound) handleR3(r3 WR3) error {

	// If we are not the root, forward r3 to parent
	if !rh.IsRoot() {
		if err := rh.SendToParent(&r3.R3); err != nil {
			return err
		}
	} else {

		// Verify reply
		if !bytes.Equal(r3.R3.HI3, rh.hash(rh.SID, rh.hash(rh.SID, rh.Leader.i2.Rc))) {
			return fmt.Errorf("R3: peer %d replied to wrong I3 message", r3.Src)
		}

		// Collect replies of the peers
		rh.Leader.r3[r3.Src] = &r3.R3

		invalid := []uint32{}
		for _, r3resp := range rh.Leader.r3[r3.Src].Responses {

			resp := &poly.Response{}
			resp.UnmarshalInit(rh.Suite())
			if err := resp.UnmarshalBinary(r3resp.Resp); err != nil {
				return err
			}

			// Verify that response is securely bound to promise and mark invalid ones
			if err := rh.Leader.states[r3resp.DealerIdx].AddResponse(int(r3resp.ShareIdx), resp); err != nil {
				invalid = append(invalid, r3resp.DealerIdx)
			}
		}
		rh.Leader.invalid[r3.Src] = &invalid

		if uint32(len(rh.Leader.r3)) == rh.Group.N-1 {
			// Continue, once all replies have arrived

			rh.Leader.i4 = &I4{
				SID:     rh.SID,
				Invalid: rh.Leader.invalid,
			}
			return rh.SendToChildren(rh.Leader.i4)
		}
	}
	return nil
}

func (rh *RandHound) handleI4(i4 WI4) error {

	// If we are not a leaf, forward i4 to children
	if !rh.IsLeaf() {
		if err := rh.SendToChildren(&i4.I4); err != nil {
			return err
		}
	}

	// Verify session ID
	if !bytes.Equal(rh.SID, i4.I4.SID) {
		return fmt.Errorf("I4: peer %d received message with incorrect session ID", rh.index())
	}

	rh.Peer.i4 = &i4.I4

	// Remove all invalid shares and prepare hash buffer
	buf := new(bytes.Buffer)
	invalid := rh.Peer.i4.Invalid[rh.index()]
	for _, dealerIdx := range *invalid {
		delete(rh.Peer.shares, dealerIdx)
		if err := binary.Write(buf, binary.LittleEndian, dealerIdx); err != nil {
			return err
		}
	}

	rh.Peer.r4 = &R4{
		Src: rh.index(),
		HI4: rh.hash(
			rh.SID,
			buf.Bytes()),
		Shares: rh.Peer.shares,
	}
	return rh.SendToParent(rh.Peer.r4)
}

func (rh *RandHound) handleR4(r4 WR4) error {

	// If we are not the root, forward r4 to parent
	if !rh.IsRoot() {
		if err := rh.SendToParent(&r4.R4); err != nil {
			return err
		}
	} else {

		// Verify reply
		buf := new(bytes.Buffer)
		invalid := rh.Leader.i4.Invalid[r4.Src]
		for _, dealerIdx := range *invalid {
			if err := binary.Write(buf, binary.LittleEndian, dealerIdx); err != nil {
				return err
			}
		}
		if !bytes.Equal(r4.R4.HI4, rh.hash(rh.SID, buf.Bytes())) {
			return fmt.Errorf("R4: peer %d replied to wrong I4 message", r4.Src)
		}

		// Collect replies of the peers
		rh.Leader.r4[r4.Src] = &r4.R4

		if uint32(len(rh.Leader.r4)) == rh.Group.N-1 {
			// Continue, once all replies have arrived

			// Process shares of i-th peer
			for i, r4 := range rh.Leader.r4 {
				for _, r4share := range r4.Shares {
					dIdx := r4share.DealerIdx
					sIdx := r4share.ShareIdx
					share := r4share.Share

					shareIdx, _ := rh.chooseTrustees(rh.Leader.rc, rh.Leader.r2[dIdx].Rs)
					if sIdx != shareIdx[i] {
						return fmt.Errorf("R4: server %d claimed share it wasn't dealt", i)
					}

					// Verify share
					if err := rh.Leader.states[dIdx].Deal.VerifyRevealedShare(int(sIdx), share); err != nil {
						return err
					}

					// Store share
					rh.Leader.states[dIdx].PriShares.SetShare(int(sIdx), share)
				}
			}

			rh.Leader.Done <- true
		}
	}
	return nil
}

package randhound

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/satori/go.uuid"
)

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

type RandHound struct {
	*sda.Node
	Leader  *Leader           // Pointer to the protocol leader
	Peer    *Peer             // Pointer to the 'current' peer
	PID     map[uuid.UUID]int // Assigns entity-uuids of peers to unique integer ids
	PKeys   []abstract.Point  // Public keys of the peers
	T       int               // Minimum number of shares needed to reconstruct the secret
	R       int               // Minimum number of signatures needed to certify a deal
	N       int               // Total number of trustees / shares (T <= R <= N)
	Purpose string            // Purpose of the protocol instance
	Done    chan bool         // For signaling that a protocol run is finished (leader only)
	Result  chan []byte       // For returning the generated randomness (leader only)
}

func NewRandHound(node *sda.Node) (sda.ProtocolInstance, error) {

	// Setup RandHound protocol struct
	rh := &RandHound{
		Node: node,
	}

	// Use TreeNode UUIDs to assign peers unique integer IDs (root node is ignored)
	j := 0
	tns := node.Tree().ListNodes()
	rh.PID = make(map[uuid.UUID]int)
	rh.PKeys = make([]abstract.Point, len(tns)-1)
	for _, t := range tns {
		if !t.IsRoot() {
			rh.PID[t.Id] = j
			rh.PKeys[j] = t.Entity.Public
			j += 1
		}
	}

	// Setup leader or peer depending on the node's location in the tree
	if node.IsRoot() {
		rh.Done = make(chan bool, 1)
		rh.Result = make(chan []byte)
		leader, err := rh.newLeader()
		if err != nil {
			return nil, err
		}
		rh.Leader = leader
	} else {
		peer, err := rh.newPeer()
		if err != nil {
			return nil, err
		}
		rh.Peer = peer
	}

	// Setup message handlers
	handlers := []interface{}{
		rh.HandleI1, rh.HandleR1,
		rh.HandleI2, rh.HandleR2,
		rh.HandleI3, rh.HandleR3,
		rh.HandleI4, rh.HandleR4,
	}
	for _, h := range handlers {
		err := rh.RegisterHandler(h)
		if err != nil {
			return nil, errors.New("Couldn't register handler: " + err.Error())
		}
	}

	return rh, nil
}

func (rh *RandHound) sendToChildren(msg interface{}) error {
	for _, c := range rh.Children() {
		err := rh.SendTo(c, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Start initiates the RandHound protocol. The leader forms the message I1 and
// sends it to  its children.
func (rh *RandHound) Start() error {
	rh.Leader.i1 = I1{
		SID:     rh.Leader.SID,
		GID:     rh.Leader.GID,
		HRc:     rh.Hash(rh.Leader.Rc),
		T:       rh.T,
		R:       rh.R,
		N:       rh.N,
		Purpose: rh.Purpose,
	}
	return rh.sendToChildren(&rh.Leader.i1)
}

// TODO: messages are currently *NOT* signed/encrypted, will be handled later automaticall by the SDA framework

func (rh *RandHound) HandleI1(i1 WI1) error {

	// If we are not a leaf, forward i1 to children
	if !rh.Node.IsLeaf() {
		err := rh.sendToChildren(&i1.I1)
		if err != nil {
			return err
		}
	}
	rh.Peer.i1 = i1.I1
	rh.T = i1.I1.T
	rh.R = i1.I1.R
	rh.N = i1.I1.N
	rh.Purpose = i1.I1.Purpose

	// TODO: verify i1 contents

	rh.Peer.r1 = R1{
		Src: rh.Peer.self,
		HI1: rh.Hash(
			rh.Peer.i1.SID,
			rh.Peer.i1.GID,
			rh.Peer.i1.HRc,
		),
		HRs: rh.Hash(rh.Peer.Rs),
	}

	return rh.SendTo(rh.Parent(), &rh.Peer.r1)
}

func (rh *RandHound) HandleR1(r1 WR1) error {

	// If we are not the root, forward r1 to parent
	if !rh.Node.IsRoot() {
		err := rh.SendTo(rh.Parent(), &r1.R1)
		if err != nil {
			return err
		}
	} else {
		// Collect replies
		rh.Leader.r1[r1.Src] = r1.R1 // TODO: streamline the message types
		rh.Leader.nr1 += 1

		// Continue, once all replies have arrived
		if rh.Leader.nr1 == len(rh.PID) {
			rh.Leader.i2 = I2{
				SID: rh.Leader.SID,
				Rc:  rh.Leader.Rc,
			}
			err := rh.sendToChildren(&rh.Leader.i2)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (rh *RandHound) HandleI2(i2 WI2) error {

	// If we are not a leaf, forward i2 to children
	if !rh.Node.IsLeaf() {
		err := rh.sendToChildren(&i2.I2)
		if err != nil {
			return err
		}
	}
	rh.Peer.i2 = i2.I2

	// TODO: verify contents of i2

	// Construct deal
	longPair := config.KeyPair{
		rh.Node.Suite(),
		rh.Node.Public(),
		rh.Node.Private(),
	}
	secretPair := config.NewKeyPair(rh.Node.Suite())
	_, insurers := rh.chooseInsurers(rh.Peer.i2.Rc, rh.Peer.Rs)
	deal := &poly.Deal{}
	deal.ConstructDeal(secretPair, &longPair, rh.T, rh.R, insurers)
	db, err := deal.MarshalBinary()
	if err != nil {
		return err
	}

	rh.Peer.r2 = R2{
		Src: rh.Peer.self,
		HI2: rh.Hash(
			rh.Peer.i2.SID,
			rh.Peer.i2.Rc),
		Rs:   rh.Peer.Rs,
		Deal: db,
	}
	return rh.SendTo(rh.Parent(), &rh.Peer.r2)
}

func (rh *RandHound) HandleR2(r2 WR2) error {

	// If we are not the root, forward r2 to parent
	if !rh.Node.IsRoot() {
		err := rh.SendTo(rh.Parent(), &r2.R2)
		if err != nil {
			return err
		}
	} else {
		// Collect replies
		rh.Leader.r2[r2.Src] = r2.R2
		rh.Leader.nr2 += 1

		rh.Leader.deals[r2.Src].UnmarshalInit(rh.T, rh.R, rh.N, rh.Node.Suite())
		if err := rh.Leader.deals[r2.Src].UnmarshalBinary(r2.Deal); err != nil {
			return err
		}

		// Continue, once all replies have arrived
		if rh.Leader.nr2 == len(rh.PID) {
			rh.Leader.i3 = I3{
				SID: rh.Leader.SID,
				R2s: rh.Leader.r2,
			}
			return rh.sendToChildren(&rh.Leader.i3)
		}
	}
	return nil
}

func (rh *RandHound) HandleI3(i3 WI3) error {

	// If we are not a leaf, forward i3 to children
	if !rh.Node.IsLeaf() {
		err := rh.sendToChildren(&i3.I3)
		if err != nil {
			return err
		}
	}
	rh.Peer.i3 = i3.I3

	longPair := config.KeyPair{
		rh.Node.Suite(),
		rh.Node.Public(),
		rh.Node.Private(),
	}

	// TODO: verify contents of i3

	r3resps := []R3Resp{}
	r4shares := []R4Share{}
	for i, r2 := range rh.Peer.i3.R2s { // NOTE: we assume that the order of R2 messages is correct since the leader took care of it

		if !bytes.Equal(r2.HI2, rh.Peer.r2.HI2) {
			return errors.New("R2 contains wrong I2 hash")
		}

		// Unmarshal Deal
		deal := &poly.Deal{}
		deal.UnmarshalInit(rh.T, rh.R, rh.N, rh.Node.Suite())
		if err := deal.UnmarshalBinary(r2.Deal); err != nil {
			return err
		}

		// Determine other peers who chose me as an insurer
		keys, _ := rh.chooseInsurers(rh.Peer.i2.Rc, r2.Rs)
		for k := range keys { // k is the share index we received from the i-th peer
			if keys[k] == rh.Peer.self {
				resp, err := deal.ProduceResponse(k, &longPair)
				if err != nil {
					return err
				}

				var r3resp R3Resp
				r3resp.Dealer = i
				r3resp.Index = k
				r3resp.Resp, err = resp.MarshalBinary()
				if err != nil {
					return err
				}
				r3resps = append(r3resps, r3resp)

				share := deal.RevealShare(k, &longPair)
				r4shares = append(r4shares, R4Share{i, k, share})
			}
		}
	}
	rh.Peer.shares = r4shares // save revealed shares for later

	rh.Peer.r3 = R3{
		Src: rh.Peer.self,
		HI3: rh.Hash(
			rh.Peer.i3.SID,
			rh.Peer.r2.HI2), // TODO: is this enough?
		Resp: r3resps,
	}
	return rh.SendTo(rh.Parent(), &rh.Peer.r3)
}

func (rh *RandHound) HandleR3(r3 WR3) error {

	// If we are not the root, forward r3 to parent
	if !rh.Node.IsRoot() {
		err := rh.SendTo(rh.Parent(), &r3.R3)
		if err != nil {
			return err
		}
	} else {
		// Collect replies
		rh.Leader.r3[r3.Src] = r3.R3
		rh.Leader.nr3 += 1

		// TODO: verify r3 contents
		for _, r3resp := range rh.Leader.r3[r3.Src].Resp {
			j := r3resp.Dealer
			_ = j

			resp := &poly.Response{}
			resp.UnmarshalInit(rh.Node.Suite())
			if err := resp.UnmarshalBinary(r3resp.Resp); err != nil {
				return err
			}
			//TODO: verify that response is securely bound to promise (how?)
		}

		// Continue, once all replies have arrived
		if rh.Leader.nr3 == len(rh.PID) {
			rh.Leader.i4 = I4{
				SID: rh.Leader.SID,
				R2s: rh.Leader.r2,
			}
			return rh.sendToChildren(&rh.Leader.i4)
		}
	}
	return nil
}

func (rh *RandHound) HandleI4(i4 WI4) error {

	// If we are not a leaf, forward i4 to children
	if !rh.Node.IsLeaf() {
		err := rh.sendToChildren(&i4.I4)
		if err != nil {
			return err
		}
	}
	rh.Peer.i4 = i4.I4

	// TODO: verify contents of i4

	rh.Peer.r4 = R4{
		Src: rh.Peer.self,
		HI4: rh.Hash(
			rh.Peer.i4.SID,
			make([]byte, 0)), // TODO: unpack R2s, see I4
		Shares: rh.Peer.shares,
	}
	return rh.SendTo(rh.Parent(), &rh.Peer.r4)
}

func (rh *RandHound) HandleR4(r4 WR4) error {

	// If we are not the root, forward r4 to parent
	if !rh.Node.IsRoot() {
		err := rh.SendTo(rh.Parent(), &r4.R4)
		if err != nil {
			return err
		}
	} else {
		// Collect replies
		rh.Leader.r4[r4.Src] = r4.R4
		rh.Leader.nr4 += 1

		// Initialise PriShares
		rh.Leader.shares[r4.Src].Empty(rh.Node.Suite(), rh.T, rh.N)

		// Continue, once all replies have arrived
		if rh.Leader.nr4 == len(rh.PID) {
			// Process shares of i-th peer
			for i, _ := range rh.Leader.r4 {
				for _, r4share := range rh.Leader.r4[i].Shares {
					j := r4share.Dealer
					idx := r4share.Index
					share := r4share.Share

					keys, _ := rh.chooseInsurers(rh.Leader.Rc, rh.Leader.r2[j].Rs)
					if keys[idx] != i {
						return errors.New(fmt.Sprintf("R4: server %d claimed share it wasn't dealt", i))
					}

					err := rh.Leader.deals[j].VerifyRevealedShare(idx, share)
					if err != nil {
						return err
					}

					// Store share
					rh.Leader.shares[j].SetShare(idx, share)
				}
			}

			// Generate the output
			output := rh.Node.Suite().Secret().Zero()
			for i := range rh.Leader.shares {
				secret := rh.Leader.shares[i].Secret()
				output.Add(output, secret)
			}

			rb, err := output.MarshalBinary()
			if err != nil {
				return err
			}

			rh.Done <- true
			rh.Result <- rb
		}
	}
	return nil
}

package randhound

import (
	"bytes"
	"errors"
	"fmt"
	"log"

	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/satori/go.uuid"
)

func init() {
	TypeI1 = network.RegisterMessageType(I1{})
	TypeR1 = network.RegisterMessageType(R1{})
	TypeI2 = network.RegisterMessageType(I2{})
	TypeR2 = network.RegisterMessageType(R2{})
	TypeI3 = network.RegisterMessageType(I3{})
	TypeR3 = network.RegisterMessageType(R3{})
	TypeI4 = network.RegisterMessageType(I4{})
	TypeR4 = network.RegisterMessageType(R4{})
	//sda.ProtocolRegisterName("RandHound", NewRandHound)
}

type RandHound struct {
	*sda.ProtocolStruct
	Leader  *Leader           // Pointer to the protocol leader
	Peer    *Peer             // Pointer to the 'current' peer
	PID     map[uuid.UUID]int // Assigns entity-uuids of peers to unique integer ids
	PKeys   []abstract.Point  // Public keys of the peers
	T       int               // Minimum number of shares needed to reconstruct the secret
	R       int               // Minimum number of signatures needed to certify a deal (t <= r <= n)
	N       int               // Total number of shares
	Purpose string            // Purpose of the protocol instance
}

func NewRandHound(h *sda.Host, t *sda.TreeNode, tok *sda.Token, T int, R int, N int, purpose string) sda.ProtocolInstance {
	if Done == nil {
		Done = make(chan bool, 1)
	}

	e, _ := h.GetEntityList(tok.EntityListID)
	el := e.List
	eid := make(map[uuid.UUID]int)
	pkeys := make([]abstract.Point, len(el)-1)

	// We ignore the leader at index 0
	for i := 1; i < len(el); i += 1 {
		j := i - 1 // adapt peer index for simpler iteration
		eid[el[i].Id] = j
		pkeys[j] = el[i].Public
	}
	return &RandHound{
		ProtocolStruct: sda.NewProtocolStruct(h, t, tok),
		Leader:         nil,
		Peer:           nil,
		PID:            eid,
		PKeys:          pkeys,
		T:              T,
		R:              R,
		N:              N,
		Purpose:        purpose,
	}
}

func (rh *RandHound) verifyMsgTypes(msgs []*sda.SDAData) (uuid.UUID, error) {
	for i := 0; i < len(msgs)-1; i += 1 {
		if msgs[i].MsgType != msgs[i+1].MsgType {
			return uuid.Nil, errors.New("Received messages of non-matching types")
		}
	}
	return msgs[0].MsgType, nil
}

func (rh *RandHound) Dispatch(msgs []*sda.SDAData) error {

	msgType, err := rh.verifyMsgTypes(msgs)
	if err != nil {
		return err
	}

	switch msgType {
	case TypeI1:
		return rh.HandleI1(msgs) // peer
	case TypeR1:
		return rh.HandleR1(msgs) // leader
	case TypeI2:
		return rh.HandleI2(msgs) // peer
	case TypeR2:
		return rh.HandleR2(msgs) // leader
	case TypeI3:
		return rh.HandleI3(msgs) // peer
	case TypeR3:
		return rh.HandleR3(msgs) // leader
	case TypeI4:
		return rh.HandleI4(msgs) // peer
	case TypeR4:
		return rh.HandleR4(msgs) // leader
	}
	return sda.NoSuchState
}

func (rh *RandHound) sendToPeers(msg network.ProtocolMessage) error {
	for _, c := range rh.Children {
		err := rh.Send(c, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Start initiates the RandHound protocol. The leader initialises itself, forms
// the message I1, and sends it to all of its peers.
func (rh *RandHound) Start() error {

	leader, err := rh.newLeader()
	if err != nil {
		return err
	}
	rh.Leader = leader

	rh.Leader.i1 = I1{
		SID: rh.Leader.SID,
		GID: rh.Leader.GID,
		HRc: rh.Hash(rh.Leader.Rc),
	}
	return rh.sendToPeers(&rh.Leader.i1)
}

// TODO: messages are currently *NOT* signed/encrypted, will be handled later automaticall by the SDA framework

// Phase 1 (peer)
func (rh *RandHound) HandleI1(msgs []*sda.SDAData) error {

	if len(msgs) > 1 {
		return errors.New("Received multiple I1-messages from the leader")
	}

	peer, err := rh.newPeer()
	if err != nil {
		return err
	}
	rh.Peer = peer
	rh.Peer.i1 = msgs[0].Msg.(I1)

	// TODO: verify i1 contents

	rh.Peer.r1 = R1{
		HI1: rh.Hash(
			rh.Peer.i1.SID,
			rh.Peer.i1.GID,
			rh.Peer.i1.HRc,
		),
		HRs: rh.Hash(rh.Peer.Rs),
	}
	return rh.Send(rh.Parent, &rh.Peer.r1)
}

// Phase 2 (leader)
func (rh *RandHound) HandleR1(msgs []*sda.SDAData) error {

	rh.Leader.r1 = make([]R1, len(rh.PID))
	for _, m := range msgs {
		r1 := m.Msg.(R1)
		i := rh.PID[m.Entity.Id]
		rh.Leader.r1[i] = r1
	}

	rh.Leader.i2 = I2{
		SID: rh.Leader.SID,
		Rc:  rh.Leader.Rc,
	}
	return rh.sendToPeers(&rh.Leader.i2)
}

// Phase 2 (peer)
func (rh *RandHound) HandleI2(msgs []*sda.SDAData) error {

	if len(msgs) > 1 {
		return errors.New("Received multiple I2-messages from the leader")
	}

	rh.Peer.i2 = msgs[0].Msg.(I2)

	// TODO: verify contents of i2

	// Construct deal
	longPair := config.KeyPair{
		rh.Host.Suite(),
		rh.Host.Entity.Public,
		rh.Host.Private(), // NOTE: the Private() function was introduced for RandHound only! Mabye there is a better solution...
	}
	secretPair := config.NewKeyPair(rh.Host.Suite())
	_, insurers := rh.chooseInsurers(rh.Peer.i2.Rc, rh.Peer.Rs, rh.Peer.self)
	deal := &poly.Deal{}
	deal.ConstructDeal(secretPair, &longPair, rh.T, rh.R, insurers)
	db, err := deal.MarshalBinary()
	if err != nil {
		return err
	}

	rh.Peer.r2 = R2{
		HI2: rh.Hash(
			rh.Peer.i2.SID,
			rh.Peer.i2.Rc),
		Rs:   rh.Peer.Rs,
		Deal: db,
	}
	return rh.Send(rh.Parent, &rh.Peer.r2)
}

// Phase 3 (leader)
func (rh *RandHound) HandleR2(msgs []*sda.SDAData) error {

	rh.Leader.r2 = make([]R2, len(rh.PID))
	rh.Leader.deals = make([]poly.Deal, len(rh.PID))
	for _, m := range msgs {
		r2 := m.Msg.(R2)
		i := rh.PID[m.Entity.Id]
		rh.Leader.r2[i] = r2

		// TODO: verify r2 contents

		// Extract and verify deal of the i-th peer
		rh.Leader.deals[i].UnmarshalInit(rh.T, rh.R, rh.N, rh.Host.Suite())
		if err := rh.Leader.deals[i].UnmarshalBinary(r2.Deal); err != nil {
			return err
		}
	}

	rh.Leader.i3 = I3{
		SID: rh.Leader.SID,
		R2s: rh.Leader.r2,
	}
	return rh.sendToPeers(&rh.Leader.i3)
}

// Phase 3 (peer)
func (rh *RandHound) HandleI3(msgs []*sda.SDAData) error {

	if len(msgs) > 1 {
		return errors.New("Received multiple I3-messages from leader")
	}

	rh.Peer.i3 = msgs[0].Msg.(I3)

	longPair := config.KeyPair{
		rh.Host.Suite(),
		rh.Host.Entity.Public,
		rh.Host.Private(),
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
		deal.UnmarshalInit(rh.T, rh.R, rh.N, rh.Host.Suite())
		if err := deal.UnmarshalBinary(r2.Deal); err != nil {
			return err
		}

		// Determine other peers who chose me as an insurer
		keys, _ := rh.chooseInsurers(rh.Peer.i2.Rc, r2.Rs, i)
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
		HI3: rh.Hash(
			rh.Peer.i3.SID,
			rh.Peer.r2.HI2), // TODO: is this enough?
		Resp: r3resps,
	}
	return rh.Send(rh.Parent, &rh.Peer.r3)
}

// Phase 3 (leader)
func (rh *RandHound) HandleR3(msgs []*sda.SDAData) error {

	rh.Leader.r3 = make([]R3, len(rh.PID))
	for _, m := range msgs {
		r3 := m.Msg.(R3)
		i := rh.PID[m.Entity.Id]
		rh.Leader.r3[i] = r3

		// TODO: verify r3 contents

		for _, r3resp := range rh.Leader.r3[i].Resp {
			j := r3resp.Dealer
			_ = j

			resp := &poly.Response{}
			resp.UnmarshalInit(rh.Host.Suite())
			if err := resp.UnmarshalBinary(r3resp.Resp); err != nil {
				return err
			}
			//TODO: verify that response is securely bound to promise (how?)
		}
	}

	rh.Leader.i4 = I4{
		SID: rh.Leader.SID,
		R2s: rh.Leader.r2,
	}
	return rh.sendToPeers(&rh.Leader.i4)
}

// Phase 4 (peer)
func (rh *RandHound) HandleI4(msgs []*sda.SDAData) error {

	if len(msgs) > 1 {
		return errors.New("Received multiple I4-messages from leader")
	}
	rh.Peer.i4 = msgs[0].Msg.(I4)

	// TODO: verify contents of i4

	rh.Peer.r4 = R4{
		HI4: rh.Hash(
			rh.Peer.i4.SID,
			make([]byte, 0)), // TODO: unpack R2s, see I4
		Shares: rh.Peer.shares,
	}
	return rh.Send(rh.Parent, &rh.Peer.r4)
}

// Phase 4 (leader)
func (rh *RandHound) HandleR4(msgs []*sda.SDAData) error {

	rh.Leader.r4 = make([]R4, len(rh.PID))
	rh.Leader.shares = make([]poly.PriShares, len(rh.PID))

	// Initialise PriShares
	for i, _ := range rh.Leader.shares {
		rh.Leader.shares[i].Empty(rh.Host.Suite(), rh.T, rh.N)
	}

	for _, m := range msgs {
		r4 := m.Msg.(R4)
		i := rh.PID[m.Entity.Id]
		rh.Leader.r4[i] = r4

		// TODO: verify r4 contents

		// Process shares of i-th peer
		for _, r4share := range r4.Shares {
			j := r4share.Dealer
			idx := r4share.Index
			share := r4share.Share

			keys, _ := rh.chooseInsurers(rh.Leader.Rc, rh.Leader.r2[j].Rs, j)
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

	output := rh.Host.Suite().Secret().Zero()
	for i := range rh.Leader.shares {
		secret := rh.Leader.shares[i].Secret()
		output.Add(output, secret)
	}

	log.Printf("RandHound - random value: %v\n", output)
	Done <- true

	return nil
}

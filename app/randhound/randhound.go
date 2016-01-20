package randhound

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
	TypeI1 = network.RegisterMessageType(I1{})
	TypeR1 = network.RegisterMessageType(R1{})
	TypeI2 = network.RegisterMessageType(I2{})
	TypeR2 = network.RegisterMessageType(R2{})
	TypeI3 = network.RegisterMessageType(I3{})
	TypeR3 = network.RegisterMessageType(R3{})
	TypeI4 = network.RegisterMessageType(I4{})
	TypeR4 = network.RegisterMessageType(R4{})
}

type ProtocolRandHound struct {
	*sda.ProtocolStruct
	Leader *Leader
	Peer   *Peer
}

func NewRandHound(h *sda.Host, t *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
	if Done == nil {
		Done = make(chan bool, 1)
	}
	return &ProtocolRandHound{
		ProtocolStruct: sda.NewProtocolStruct(h, t, tok),
		Leader:         nil,
		Peer:           nil,
	}
}

func (p *ProtocolRandHound) Dispatch(m []*sda.SDAData) error {
	switch m[0].MsgType {
	case TypeI1:
		return p.HandleI1(m[0]) // peer
	case TypeR1:
		return p.HandleR1(m) // leader
	case TypeI2:
		return p.HandleI2(m[0]) // peer
	case TypeR2:
		return p.HandleR2(m) // leader
	case TypeI3:
		return p.HandleI3(m[0]) // peer
	case TypeR3:
		return p.HandleR3(m) // leader
	case TypeI4:
		return p.HandleI4(m[0]) // peer
	case TypeR4:
		return p.HandleR4(m) // leader
	}
	return sda.NoSuchState
}

// Start initiates the RandHound protocol. The leader initialises itself, forms
// the message I1, and sends it to all of its peers.
func (p *ProtocolRandHound) Start() error {

	leader, err := p.newLeader()
	if err != nil {
		return err
	}
	p.Leader = leader

	p.Leader.i1 = I1{
		SID: p.Leader.SID,
		GID: p.Leader.GID,
		HRc: p.Hash(p.Leader.Rc),
	}

	for _, c := range p.Children {
		err := p.Send(c, &p.Leader.i1)
		if err != nil {
			return err
		}
	}

	return nil
}

// Ix: Messages from leader to peers
// Rx: Messages from peer to leader
// TODO: find better name for the handle functions since they basically include: receive msg, operation, send msg
// TODO: messages are currently *NOT* signed/encrypted, will be handled later automaticall by the SDA framework
// suite := p.ProtocolStruct.Host.Suite()

// Phase 1 (peer)
func (p *ProtocolRandHound) HandleI1(m *sda.SDAData) error {

	p.Peer = p.newPeer()
	p.Peer.i1 = m.Msg.(I1)

	// TODO: verify i1 contents

	p.Peer.r1 = R1{
		HI1: p.Hash(
			p.Peer.i1.SID,
			p.Peer.i1.GID,
			p.Peer.i1.HRc,
		),
		HRs: p.Hash(p.Peer.Rs)}

	return p.Send(p.Parent, &p.Peer.r1)
}

//
// Phase 2 (leader)
func (p *ProtocolRandHound) HandleR1(m []*sda.SDAData) error {

	p.Leader.r1 = make([]R1, len(m))
	for i, _ := range m {
		p.Leader.r1[i] = m[i].Msg.(R1)
		// TODO: verify r1 contents
	}

	p.Leader.i2 = I2{
		SID: p.Leader.SID,
		Rc:  p.Leader.Rc}

	for _, c := range p.Children {
		err := p.Send(c, &p.Leader.i2)
		if err != nil {
			return err
		}
	}

	return nil
}

//
// Phase 2 (peer)
func (p *ProtocolRandHound) HandleI2(m *sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	p.Peer.i2 = m.Msg.(I2)

	// TODO: verify contents of i2

	// TODO: Construct deal

	Deal := make([]byte, 0)
	p.Peer.r2 = R2{
		HI2: p.Hash(
			p.Peer.i2.SID,
			p.Peer.i2.Rc),
		Rs:   p.Peer.Rs,
		Deal: Deal}

	return p.Send(p.Parent, &p.Peer.r2)
}

// Phase 3 (leader)
func (p *ProtocolRandHound) HandleR2(m []*sda.SDAData) error {

	p.Leader.r2 = make([]R2, len(m))
	for i, _ := range m {
		p.Leader.r2[i] = m[i].Msg.(R2)
		// TODO: verify r2 contents
		// TODO: store r2 in transcript
		// TODO: deal processing
	}

	R2s := make([][]byte, 0)
	p.Leader.i3 = I3{
		SID: p.Leader.SID,
		R2s: R2s}

	for _, c := range p.Children {
		err := p.Send(c, &p.Leader.i3)
		if err != nil {
			return err
		}
	}

	return nil
}

// Phase 3 (peer)
func (p *ProtocolRandHound) HandleI3(m *sda.SDAData) error {

	p.Peer.i3 = m.Msg.(I3)

	// TODO: verify contents of i3

	// TODO: do magic

	p.Peer.r3 = R3{
		HI3: p.Hash(
			p.Peer.i3.SID,
			make([]byte, 0)), // TODO: unpack R2s, see I3
		Resp: make([]R3Resp, 0)}

	return p.Send(p.Parent, &p.Peer.r3)
}

// Phase 3 (leader)
func (p *ProtocolRandHound) HandleR3(m []*sda.SDAData) error {

	p.Leader.r3 = make([]R3, len(m))
	for i := range m {
		p.Leader.r3[i] = m[i].Msg.(R3)
		// TODO: verify r3 contents
		// TODO: store r3 in transcript
		// TODO: do magic
	}

	R2s := make([][]byte, 0)
	p.Leader.i4 = I4{
		SID: p.Leader.SID,
		R2s: R2s}

	for _, c := range p.Children {
		err := p.Send(c, &p.Leader.i4)
		if err != nil {
			return err
		}
	}

	return nil
}

// Phase 4 (peer)
func (p *ProtocolRandHound) HandleI4(m *sda.SDAData) error {

	p.Peer.i4 = m.Msg.(I4)

	// TODO: verify contents of i4

	// TODO: do magic

	p.Peer.r4 = R4{
		HI4: p.Hash(
			p.Peer.i4.SID,
			make([]byte, 0)), // TODO: unpack R2s, see I4
		Shares: make([]R4Share, 0)}

	return p.Send(p.Parent, &p.Peer.r4)
}

// Phase 4 (leader)
func (p *ProtocolRandHound) HandleR4(m []*sda.SDAData) error {

	p.Leader.r4 = make([]R4, len(m))
	for i := range m {
		p.Leader.r4[i] = m[i].Msg.(R4)
		// TODO: verify r4 contents
		// TODO: store r4 in transcript
		// TODO: do magic
	}

	// TODO: reconstruct final secret and print the random number
	Done <- true
	dbg.Lvl1("The public random number is:", 0)

	return nil
}

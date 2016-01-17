package randhound

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/random"
)

var Done chan bool
var TypeI1 = network.RegisterMessageType(I1{})
var TypeR1 = network.RegisterMessageType(R1{})
var TypeI2 = network.RegisterMessageType(I2{})
var TypeR2 = network.RegisterMessageType(R2{})
var TypeI3 = network.RegisterMessageType(I3{})
var TypeR3 = network.RegisterMessageType(R3{})
var TypeI4 = network.RegisterMessageType(I4{})
var TypeR4 = network.RegisterMessageType(R4{})

func init() {
	dbg.Lvl1("I got called!")
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

type ProtocolRandHound struct {
	*sda.ProtocolStruct
	Leader Leader
	Peer   Peer
}

func NewRandHound(h *sda.Host, t *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
	if Done == nil {
		Done = make(chan bool, 1)
	}
	return &ProtocolRandHound{
		ProtocolStruct: sda.NewProtocolStruct(h, t, tok),
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

// Start initiates the RandHound protocol. The leader forms the message I1 and
// sends it to all of its peers.
func (p *ProtocolRandHound) Start() error {

	//suite := p.ProtocolStruct.Host.Suite()
	//_ = suite

	// TODO: this has to go into the init of the leader
	x, _ := p.ProtocolStruct.Host.Entity.Public.MarshalBinary()
	session := &Session{x, "RandHound test run", time.Now()}

	// Compute SID (TODO: error checking)
	y := []byte(session.Purpose)
	z, _ := session.Time.MarshalBinary()
	sid := p.Hash(x, y, z) // TODO: SID should probably be a field of the Leader struct

	// Compute GID (TODO: marshal and include public keys of the peers, error checking)
	group := &Group{make([][]byte, 0), 1, 2, 3, 4}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, group.F)
	binary.Write(buf, binary.LittleEndian, group.L)
	binary.Write(buf, binary.LittleEndian, group.K)
	binary.Write(buf, binary.LittleEndian, group.T)
	gid := p.Hash(buf.Bytes()) // TODO: GID should probably be a field of the Leader struct

	// Choose client's trustee-selection randomness
	keysize := 32               // TODO: store in Leader struct
	rc := make([]byte, keysize) // TODO: store in Leader struct
	random.Stream.XORKeyStream(rc, rc)
	hrc := p.Hash(rc)

	//dbg.Lvl1("SID:", sid)
	//dbg.Lvl1("GID:", gid)
	//dbg.Lvl1("HRc:", hrc)

	i1 := I1{SID: sid, GID: gid, HRc: hrc, S: make([]byte, 0), G: make([]byte, 0)}
	for _, c := range p.Children {
		dbg.Lvl1("Sending msg to client:", c)
		err := p.Send(c, &i1)
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

// Phase 1 (peer)
func (p *ProtocolRandHound) HandleI1(m *sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	i1 := m.Msg.(I1)
	_, _ = suite, i1

	// TODO: verify i1 contents
	//dbg.Lvl1("Received I1:", i1)

	// Choose peer's trustee-selection randomness
	keysize := 32 // TODO: move to Peer struct
	rs := make([]byte, keysize)
	random.Stream.XORKeyStream(rs, rs)
	hrs := p.Hash(rs)

	r1 := R1{HI1: p.Hash(i1.SID, i1.GID, i1.HRc, i1.S, i1.G), HRs: hrs}
	dbg.Lvl1("R1:", r1)
	return p.Send(p.Parent, &r1)
}

//
// Phase 2 (leader)
func (p *ProtocolRandHound) HandleR1(m []*sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	_ = suite

	for i := range m {
		r1 := m[i].Msg.(R1)
		_ = r1
		//dbg.Lvl1("Received R1:", r1)
		// TODO: verify r1 contents
		// TODO: store r1 in transcript
	}

	// TODO: do magic

	sid := make([]byte, 0)
	Rc := make([]byte, 0)
	i2 := I2{SID: sid, Rc: Rc}
	for _, c := range p.Children {
		err := p.Send(c, &i2)
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
	i2 := m.Msg.(I2)
	_, _ = i2, suite

	// TODO: verify contents of i2

	// TODO: Construct deal

	HI2 := make([]byte, 0)
	Rs := make([]byte, 0)
	Deal := make([]byte, 0)
	r2 := R2{HI2: HI2, Rs: Rs, Deal: Deal}
	return p.Send(p.Parent, &r2)
}

// Phase 3 (leader)
func (p *ProtocolRandHound) HandleR2(m []*sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	_ = suite

	for i := range m {
		r2 := m[i].Msg.(R2)
		_ = r2
		// TODO: verify r2 contents
		// TODO: store r2 in transcript
		// TODO: deal processing
	}

	sid := make([]byte, 0)
	R2s := make([][]byte, 0)
	i3 := I3{SID: sid, R2s: R2s}
	for _, c := range p.Children {
		err := p.Send(c, &i3)
		if err != nil {
			return err
		}
	}
	return nil
}

// Phase 3 (peer)
func (p *ProtocolRandHound) HandleI3(m *sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	i3 := m.Msg.(I3)
	_, _ = suite, i3

	// TODO: verify contents of i3

	// TODO: do magic

	HI3 := make([]byte, 0)
	r3 := R3{HI3: HI3, Resp: make([]R3Resp, 0)}
	return p.Send(p.Parent, &r3)
}

// Phase 3 (leader)
func (p *ProtocolRandHound) HandleR3(m []*sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	_ = suite

	for i := range m {
		r3 := m[i].Msg.(R3)
		_ = r3
		// TODO: verify r3 contents
		// TODO: store r3 in transcript
		// TODO: do magic
	}

	sid := make([]byte, 0)
	R2s := make([][]byte, 0)
	i4 := I4{SID: sid, R2s: R2s}
	for _, c := range p.Children {
		err := p.Send(c, &i4)
		if err != nil {
			return err
		}
	}
	return nil
}

// Phase 4 (peer)
func (p *ProtocolRandHound) HandleI4(m *sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	i4 := m.Msg.(I4)
	_, _ = i4, suite

	// TODO: verify contents of i4

	// TODO: do magic

	HI4 := make([]byte, 0)
	r4 := R4{HI4: HI4, Shares: make([]R4Share, 0)}
	return p.Send(p.Parent, &r4)
}

// Phase 4 (leader)
func (p *ProtocolRandHound) HandleR4(m []*sda.SDAData) error {

	suite := p.ProtocolStruct.Host.Suite()
	_ = suite

	for i := range m {
		dbg.Lvl1("Receiving message:", m[i])
		r4 := m[i].Msg.(R4)
		_ = r4
		// TODO: verify r4 contents
		// TODO: store r4 in transcript
		// TODO: do magic
	}

	// TODO: reconstruct final secret and print the random number
	Done <- true
	dbg.Lvl1("The public random number is:", 0)
	return nil
}

package randhound

import "github.com/dedis/cothority/lib/sda"

func init() {
	sda.ProtocolRegister("RandHound", NewProtocolInstance)
}

type ProtocolRandHound struct {
	sda.Host
	sda.TreeNode
	c Client
	s Server
}

func NewProtocolInstance(h *sda.Host, t *sda.TreeNode) *ProtocolRandHound {
	return &ProtocolRandHound{
		Host:     h,
		TreeNode: t,
		Client:   c,
		Server:   s,
	}
}

// Start initiates the RandHound protocol by forming the message I1 on
// the side of the client and sending it to the servers
func (p *ProtocolRandHound) Start() error {

	suite := p.sda.Host.suite
	_ = suite

	//public key of the host: p.sda.Host.Entity.Public
	// TODO: figure out session and group marshalling

	sid := make([]byte, 0)
	gid := make([]byte, 0)
	HRc := make([]byte, 0)
	S := make([]byte, 0)
	G := make([]byte, 0)
	p.c.i1 = I1{SID: sid, GID: gid, HRc: HRc, S: S, G: G}
	for nil, c := range p.Children() {
		err := p.SendMsgTo(c, p.c.i1)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *ProtocolRandHound) Dispatch(m []*sda.SDAData) error {
	switch m[0].MsgType {
	case 0:
		return p.HandleI1R1(m[0]) // server
	case 1:
		return p.HandleR1I2(m) // client
	case 2:
		return p.HandleI2R2(m[0]) // server
	case 3:
		return p.HandleR2I3(m) // client
	case 4:
		return p.HandleI3R3(m[0]) // server
	case 5:
		return p.HandleR3I4(m) // client
	case 6:
		return p.HandleI4R4(m[0]) // server
	case 7:
		return p.HandleR4(m) // client
	}
	return sda.NoSuchState
}

// Ix: Messages from client to server
// Rx: Messages from server to client
// TODO: rename client/server to leader/node or leader/follower to be consistent with the rest of the project?
// TODO: find better name for the handle functions since they basically include: receive msg, operation, send msg
// TODO: messages are currently *NOT* signed/encrypted, will be handled later automaticall by the SDA framework

// Phase 1 (server)
func (p *ProtocolRandHound) HandleI1R1(m *sda.SDAData) error {

	//suite := p.sda.Host.suite
	i1 := m.Msg.(I1)
	_ = i1

	// TODO: verify i1 contents

	// Choose server's trustee-selection randomness
	Rs := make([]byte, p.s.keysize)
	p.s.rand.XORKeyStream(Rs, Rs)

	// Form R1 and send it to client

	HI1 := make([]byte, 0)
	HRs := make([]byte, 0)
	r1 := R1{HI1: HI1, HRs: HRs}
	return p.SendMsgTo(p.Parent(), r1)
}

// Phase 2 (client)
func (p *ProtocolRandHound) HandleR1I2(m []*sda.SDAData) error {

	suite := p.sda.Host.suite
	_ = suite

	for i := range m {
		r1 := m[i].Msg.(R1)
		_ = r1
		// TODO: verify r1 contents
		// TODO: store r1 in transcript
	}

	// TODO: do magic

	sid := make([]byte, 0)
	Rc := make([]byte, 0)
	p.c.i2 = I2{SID: sid, Rc: Rc}
	for nil, c := range p.Children() {
		err := p.SendMsgTo(c, p.c.i2)
		if err != nil {
			return err
		}
	}
	return nil
}

// Phase 2 (server)
func (p *ProtocolRandHound) HandleI2R2(m *sda.SDAData) error {

	suite := p.sda.Host.suite
	i2 := m.Msg.(I2)
	_, _ = i2, suite

	// TODO: verify contents of i2

	// TODO: Construct deal

	HI2 := make([]byte, 0)
	Rs := make([]byte, 0)
	Deal := make([]byte, 0)
	r2 := R2{HI2: HI2, Rs: Rs, Deal: Deal}
	return p.SendMsgTo(p.Parent(), r2)
}

// Phase 3 (client)
func (p *ProtocolRandHound) HandleR2I3(m []*sda.SDAData) error {

	suite := p.sda.Host.suite
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
	p.c.i3 = I3{SID: sid, R2s: R2s}
	for nil, c := range p.Children() {
		err := p.SendMsgTo(c, p.c.i3)
		if err != nil {
			return err
		}
	}
	return nil
}

// Phase 3 (server)
func (p *ProtocolRandHound) HandleI3R3(m *sda.SDAData) error {

	//suite := p.sda.Host.suite
	i3 := m.Msg.(I3)
	_ = i3

	// TODO: verify contents of i3

	// TODO: do magic

	HI3 := make([]byte, 0)
	R3Resp := R3Resp{0, 0, make([]byte, 0)}
	r3 := R3{HI3: HI3, R3Resp}
	return p.SendMsgTo(p.Parent(), r3)
}

// Phase 3 (client)
func (p *ProtocolRandHound) HandleR3I4(m []*sda.SDAData) error {

	suite := p.sda.Host.suite
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
	p.c.i4 = I4{SID: sid, R2s: R2s}
	for nil, c := range p.Children() {
		err := p.SendMsgTo(c, p.c.i4)
		if err != nil {
			return err
		}
	}
	return nil
}

// Phase 4 (server)
func (p *ProtocolRandHound) HandleI4R4(m *sda.SDAData) error {

	suite := p.sda.Host.suite
	i4 := m.Msg.(I4)
	_, _ = i4, suite

	// TODO: verify contents of i4

	// TODO: do magic

	HI4 := make([]byte, 0)
	R4Share := R4Share{0, 0, nil}
	r4 := R4{HI4: HI4, R4Share}
	return p.SendMsgTo(p.Parent(), r4)
}

// Phase 4 (client)
func (p *ProtocolRandHound) HandleR4(m []*sda.SDAData) error {

	suite := p.sda.Host.suite
	_ = suite

	for i := range len(m) {
		r4 := m[i].Msg.(R4)
		_ = r4
		// TODO: verify r4 contents
		// TODO: store r4 in transcript
		// TODO: do magic
	}

	// TODO: reconstruct final secret and print it

	return nil
}

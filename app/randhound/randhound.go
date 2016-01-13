package randhound

import "github.com/dedis/cothority/lib/sda"

func init() {
	sda.ProtocolRegister("RandHound", NewProtocolInstance)
}

type ProtocolRandHound struct {
	sda.Host
	sda.TreeNode
	c client
	s server
}

func NewProtocolInstance(h *sda.Host, t *sda.TreeNode) *ProtocolRandHound {
	return &ProtocolRandHound{
		Host:     h,
		TreeNode: t,
		Client:   c,
		Server:   s,
	}
}

func (p *ProtocolRandHound) Start() error {

	// start the protocol by forming the first message I1 on client-side and send it to the servers

	return nil
}

func (p *ProtocolRandHound) Dispatch(m []*sda.SDAData) error {
	switch m[0].MsgType {
	case 0:
		return p.HandleI1(m)
	case 1:
		return p.HandleR1(m)
	case 2:
		return p.HandleI2(m)
	case 3:
		return p.HandleR2(m)
	case 4:
		return p.HandleI3(m)
	case 5:
		return p.HandleR3(m)
	case 6:
		return p.HandleI4(m)
	case 7:
		return p.HandleR4(m)
	}
	return sda.NoSuchState
}

// Ix: Messages from client to server
// Rx: Messages from server to client
// TODO: rename client/server to leader/node or leader/follower to be consistent with the rest of the project?

// Phase 1
func (p *ProtocolRandHound) HandleI1(m *sda.SDAData) error {

	// Server: receive I1, validate+process it, form and send R1

	return nil
}

func (p *ProtocolRandHound) HandleR1(m *sda.SDAData) error {

	// Client: receive R1, validate+process it, form and send I2

	return nil
}

// Phase 2
func (p *ProtocolRandHound) HandleI2(m *sda.SDAData) error {

	// Server: receive I2, validate+process it, form and send R2

	return nil
}

func (p *ProtocolRandHound) HandleR2(m *sda.SDAData) error {

	// Client: receive R2, validate+process it, form and send I3

	return nil
}

// Phase 3
func (p *ProtocolRandHound) HandleI3(m *sda.SDAData) error {

	// Server: receive I3, validate+process it, form and send R3

	return nil
}

func (p *ProtocolRandHound) HandleR3(m *sda.SDAData) error {

	// Client: receive R3, validate+process it, form and send I4

	return nil
}

// Phase 4
func (p *ProtocolRandHound) HandleI4(m *sda.SDAData) error {

	// Server: receive I4, validate+process it, form and send R4

	return nil
}

func (p *ProtocolRandHound) HandleR4(m *sda.SDAData) error {

	// Client: receive R4, validate+process it, output public random value

	return nil
}

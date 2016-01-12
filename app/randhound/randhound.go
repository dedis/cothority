package randhound

import "github.com/dedis/cothority/lib/sda"

func init() {
	sda.ProtocolRegister("RandHound", NewProtocolInstance)
}

type ProtocolRandHound struct {
	sda.Host
	sda.TreeNode
}

// TODO: define the 8 RandHound messages

func NewProtocolInstance(h *sda.Host, t *sda.TreeNode) *ProtocolRandHound {
	return &ProtocolRandHound{
		Host:     h,
		TreeNode: t,
	}
}

func (p *ProtocolRandHound) Dispatch(m []*sda.SDAData) error {
	switch m[0].MsgType {
	case 0:
		return p.HandleI0(m)
	case 1:
		return p.HandleI1(m)
	case 2:
		return p.HandleI2(m)
	case 3:
		return p.HandleI3(m)
	case 4:
		return p.HandleR0(m)
	case 5:
		return p.HandleR1(m)
	case 6:
		return p.HandleR2(m)
	case 7:
		return p.HandleR3(m)
	}
	return sda.NoSuchState
}

// Ix: Messages from client to server
func (*ProtocolRandHound) HandleI0(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleI1(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleI2(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleI3(m *sda.SDAData) error {
	return nil
}

// Rx: Messages from server to client
func (*ProtocolRandHound) HandleR0(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleR1(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleR2(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleR3(m *sda.SDAData) error {
	return nil
}

package randhound

import "github.com/dedis/cothority/lib/sda"

func init() {
	sda.ProtocolRegister("RandHound", NewProtocolInstance)
}

type ProtocolRandHound struct {
	sda.Host
	sda.TreeNode
}

func NewProtocolInstance(h *sda.Host, t *sda.TreeNode) *ProtocolRandHound {
	return &ProtocolRandHound{
		Host:     h,
		TreeNode: t,
	}
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
func (*ProtocolRandHound) HandleI1(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleR1(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleI2(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleR2(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleI3(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleR3(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleI4(m *sda.SDAData) error {
	return nil
}

func (*ProtocolRandHound) HandleR4(m *sda.SDAData) error {
	return nil
}

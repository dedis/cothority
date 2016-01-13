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
// TODO: rename client/server to leader/node or leader/follower to be consistent with the rest of the project?

// Phase 1
func (p *ProtocolRandHound) HandleI1(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 1 (client): form I1 message and send it to all servers
	} else {
		// Step 2 (server): receive I1 and process (=validate) it
	}
	return nil
}

func (p *ProtocolRandHound) HandleR1(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 4 (client): receive R1 and process it
	} else {
		// Step 3 (server): choose server's trustee-selection randomness, form R1 and send it to client
	}
	return nil
}

// Phase 2
func (p *ProtocolRandHound) HandleI2(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 5 (client): form I2 message and send it to all servers
	} else {
		// Step 6 (server): receive I2, extract Rc, compute HRc and verify it
	}
	return nil
}

func (p *ProtocolRandHound) HandleR2(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 8 (client): receive R2 and process it
	} else {
		// Step 7 (server): construct deal, form R2 and send it to the client
	}
	return nil
}

// Phase 3
func (p *ProtocolRandHound) HandleI3(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 9 (client): form I3 and send it to the servers
	} else {
		// Step 10 (server): receive I3 and verify it
	}
	return nil
}

func (p *ProtocolRandHound) HandleR3(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 12 (client): receive R3 and process it
	} else {
		// Step 11 (server): Decrypt and validate all the shares we've been dealt, form R3, and send it to the client
	}
	return nil
}

// Phase 4
func (p *ProtocolRandHound) HandleI4(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 13 (client): form I4 and send it to the servers
	} else {
		// Step 14 (server): receive R4, verify it
	}
	return nil
}

func (p *ProtocolRandHound) HandleR4(m *sda.SDAData) error {
	if p.IsRoot() {
		// Step 16 (client): receive R3 and reconstruct the final secret (print output value to log for debugging)
	} else {
		// Step 15 (server): form R3 and send it to client
	}
	return nil
}

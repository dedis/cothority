package randhound

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.ProtocolRegister("RandHound", NewProtocolInstance)
}

type ProtocolRandHound struct {
	sda.Host
	sda.TreeNode
}

// TODO: it's probably better to extract the message types into a separate file

type I1 struct {
	SID []byte // Session identifier: hash of session info block
	GID []byte // Group identifier: hash of group parameter block
	HRc []byte // Client's trustee-randomness commit
	S   []byte // Full session info block (optional)
	G   []byte // Full group parameter block (optional)
}

type R1 struct {
	HI1 []byte // Hash of I1 message
	HRs []byte // Server's trustee-randomness commit
}

type I2 struct {
	SID []byte // Session identifier
	Rc  []byte // Client's trustee-selection randomness
}

type R2 struct {
	HI2  []byte // Hash of I2 message
	Rs   []byte // Servers' trustee-selection randomness
	Deal []byte // Server's secret-sharing to trustees
}

type I3 struct {
	SID []byte   // Session identifier
	R2s [][]byte // Client's list of signed R2 messages; empty slices represent missing R2 messages
}

type R3 struct {
	HI3  []byte   // Hash of I3 message
	Resp []R3Resp // Responses to dealt secret-shares
}

type R3Resp struct {
	Dealer int    // Server number of dealer
	Index  int    // Share number in deal we are validating
	Resp   []byte // Encoded response to dealer's Deal
}

type I4 struct {
	SID []byte   // Session identifier
	R2s [][]byte // Client's list of signed R2 messages; empty slices represent missing R2 messages
}

type R4 struct {
	HI4    []byte    // Hash of I4 message
	Shares []R4Share // Revealed secret-shares
}

type R4Share struct {
	Dealer int             // Server number of dealer
	Index  int             // Share number in dealer's Deal
	Share  abstract.Secret // Decrypted share dealt to this server
}

// IMessage represents any message a RandHound initiator/client can send.
// The fields of this struct should be treated as a protobuf 'oneof'.
type IMessage struct {
	I1 *I1
	I2 *I2
	I3 *I3
	I4 *I4
}

// RMessage represents any message a RandHound responder/server can send.
// The fields of this struct should be treated as a protobuf 'oneof'.
type RMessage struct {
	RE *RError
	R1 *R1
	R2 *R2
	R3 *R3
	R4 *R4
}

type Transcript struct {
	I1 []byte   // I1 message signed by client
	R1 [][]byte // R1 messages signed by resp servers
	I2 []byte
	R2 [][]byte
	I3 []byte
	R3 [][]byte
	I4 []byte
	R4 [][]byte
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

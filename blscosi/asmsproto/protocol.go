// Package asmsproto implements the Accountable-Subgroup Multi-Signatures over BLS
// to protect the aggregates against rogue public-key attacks. This is a modified
// version of blscosi/protocol which is now deprecated.
package asmsproto

import (
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/asmbls"
	"go.dedis.ch/onet/v3"
)

// AsmsProtocolName is the name of the main protocol for the modified BLS signature scheme
const AsmsProtocolName = "asmsCoSiProto"

// AsmsSubProtocolName is the name of the subprotocol for the modified BLS signature scheme
const AsmsSubProtocolName = "asmsSubCosiProto"

func init() {
	GlobalRegisterAsmsProtocols()
}

// GlobalRegisterAsmsProtocols registers both protocol to the global register
func GlobalRegisterAsmsProtocols() {
	onet.GlobalProtocolRegister(AsmsProtocolName, NewAsmsProtocol)
	onet.GlobalProtocolRegister(AsmsSubProtocolName, NewSubAsmsProtocol)
}

// NewAsmsProtocol is used to register the protocol with an always-true verification
func NewAsmsProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewAsmsCosi(n, vf, AsmsSubProtocolName, pairing.NewSuiteBn256())
}

// NewAsmsCosi makes a protocol instance for the modified BLS CoSi protocol
func NewAsmsCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, subProtocolName string, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	c, err := protocol.NewBlsCosi(n, vf, subProtocolName, suite)
	if err != nil {
		return nil, err
	}

	mbc := c.(*protocol.BlsCosi)
	mbc.Sign = asmbls.Sign
	mbc.Verify = asmbls.Verify
	mbc.Aggregate = aggregate

	return mbc, nil
}

// NewSubAsmsProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewSubAsmsProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubAsmsCosi(n, vf, pairing.NewSuiteBn256())
}

// NewSubAsmsCosi uses the default sub-protocol to make one compatible with
// the robust scheme
func NewSubAsmsCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	pi, err := protocol.NewSubBlsCosi(n, vf, suite)
	if err != nil {
		return nil, err
	}

	subCosi := pi.(*protocol.SubBlsCosi)
	subCosi.Sign = asmbls.Sign
	subCosi.Verify = asmbls.Verify
	subCosi.Aggregate = aggregate

	return subCosi, nil
}

// aggregate uses the robust aggregate algorithm to aggregate the signatures
func aggregate(suite pairing.Suite, mask *sign.Mask, sigs [][]byte) ([]byte, error) {
	sig, err := asmbls.AggregateSignatures(suite, sigs, mask)
	if err != nil {
		return nil, err
	}

	return sig.MarshalBinary()
}

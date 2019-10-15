// Package bdnproto implements the Boneh-Drijvers-Neven signature scheme
// to protect the aggregates against rogue public-key attacks.
// This is a modified version of blscosi/protocol which is now deprecated.
package bdnproto

import (
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/kyber/v4/sign/bdn"
	"go.dedis.ch/onet/v4"
)

// BdnProtocolName is the name of the main protocol for the BDN signature scheme.
const BdnProtocolName = "bdnCoSiProto"

// BdnSubProtocolName is the name of the subprotocol for the BDN signature scheme.
const BdnSubProtocolName = "bdnSubCosiProto"

// GlobalRegisterBdnProtocols registers both protocol to the global register.
func GlobalRegisterBdnProtocols() {
	onet.GlobalProtocolRegister(BdnProtocolName, NewBdnProtocol)
	onet.GlobalProtocolRegister(BdnSubProtocolName, NewSubBdnProtocol)
}

// NewBdnProtocol is used to register the protocol with an always-true verification.
func NewBdnProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBdnCosi(n, vf, BdnSubProtocolName, pairing.NewSuiteBn256())
}

// NewBdnCosi makes a protocol instance for the BDN CoSi protocol.
func NewBdnCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, subProtocolName string, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	c, err := protocol.NewBlsCosi(n, vf, subProtocolName, suite)
	if err != nil {
		return nil, err
	}

	mbc := c.(*protocol.BlsCosi)
	mbc.Sign = bdn.Sign
	mbc.Verify = bdn.Verify
	mbc.Aggregate = aggregate

	return mbc, nil
}

// NewSubBdnProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewSubBdnProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubBdnCosi(n, vf, pairing.NewSuiteBn256())
}

// NewSubBdnCosi uses the default sub-protocol to make one compatible with
// the robust scheme.
func NewSubBdnCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	pi, err := protocol.NewSubBlsCosi(n, vf, suite)
	if err != nil {
		return nil, err
	}

	subCosi := pi.(*protocol.SubBlsCosi)
	subCosi.Sign = bdn.Sign
	subCosi.Verify = bdn.Verify
	subCosi.Aggregate = aggregate

	return subCosi, nil
}

// aggregate uses the robust aggregate algorithm to aggregate the signatures.
func aggregate(suite pairing.Suite, mask *sign.Mask, sigs [][]byte) ([]byte, error) {
	sig, err := bdn.AggregateSignatures(suite, sigs, mask)
	if err != nil {
		return nil, err
	}

	return sig.MarshalBinary()
}

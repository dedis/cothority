// Package bdnproto implements the Boneh-Drijvers-Neven signature scheme
// to protect the aggregates against rogue public-key attacks.
// This is a modified version of blscosi/protocol which is now deprecated.
package bdnproto

import (
	"go.dedis.ch/cothority/v4/blscosi/protocol"
	"go.dedis.ch/cothority/v4/cosuite"
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
	return NewBdnCosi(n, vf, BdnSubProtocolName, cosuite.NewBdnSuite())
}

// NewBdnCosi makes a protocol instance for the BDN CoSi protocol.
func NewBdnCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, subProtocolName string, suite cosuite.CoSiCipherSuite) (onet.ProtocolInstance, error) {
	return protocol.NewBlsCosi(n, vf, subProtocolName, suite)
}

// NewSubBdnProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewSubBdnProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubBdnCosi(n, vf, cosuite.NewBdnSuite())
}

// NewSubBdnCosi uses the default sub-protocol to make one compatible with
// the robust scheme.
func NewSubBdnCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, suite cosuite.CoSiCipherSuite) (onet.ProtocolInstance, error) {
	pi, err := protocol.NewSubBlsCosi(n, vf, suite)
	if err != nil {
		return nil, err
	}

	subCosi := pi.(*protocol.SubBlsCosi)

	return subCosi, nil
}

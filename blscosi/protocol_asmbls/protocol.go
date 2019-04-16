package protocol2

import (
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/asmbls"
	"go.dedis.ch/onet/v3"
)

// ModifiedProtocolName is the name of the main protocol for the modified BLS signature scheme
const ModifiedProtocolName = "blsCoSiProtoModified"

// ModifiedSubProtocolName is the name of the subprotocol for the modified BLS signature scheme
const ModifiedSubProtocolName = "blsSubCosiProtoModified"

func init() {
	GlobalRegisterModifiedProtocols()
}

// GlobalRegisterModifiedProtocols registers both protocol to the global register
func GlobalRegisterModifiedProtocols() {
	onet.GlobalProtocolRegister(ModifiedProtocolName, NewModifiedProtocol)
	onet.GlobalProtocolRegister(ModifiedSubProtocolName, NewModifiedSubProtocol)
}

// NewModifiedProtocol is used to register the protocol with an always-true verification
func NewModifiedProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewModifiedBlsCosi(n, vf, ModifiedSubProtocolName, pairing.NewSuiteBn256())
}

// ModifiedBlsCosi is the extended version of BlsCosi that is using a robust aggregate scheme
type ModifiedBlsCosi struct {
	*protocol.BlsCosi

	FinalSignature chan ModifiedBlsSignature
}

// NewModifiedBlsCosi makes a protocol instance for the modified BLS CoSi protocol
func NewModifiedBlsCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, subProtocolName string, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	c, err := protocol.NewBlsCosi(n, vf, subProtocolName, suite)
	if err != nil {
		return nil, err
	}

	mbc := &ModifiedBlsCosi{
		BlsCosi:        c.(*protocol.BlsCosi),
		FinalSignature: make(chan ModifiedBlsSignature),
	}
	mbc.Sign = asmbls.Sign
	mbc.Verify = asmbls.Verify
	mbc.Aggregate = aggregate

	return mbc, nil
}

// Dispatch reuses the default protocol behaviour but hooks the result and cast into
// a signature that is verified using the robust aggregate scheme
func (p *ModifiedBlsCosi) Dispatch() error {
	err := p.BlsCosi.Dispatch()
	if err != nil {
		return err
	}

	// The previous dispatch ends correctly only after having written the final
	// signature so either it will return an error or this won't hang
	sig := <-p.BlsCosi.FinalSignature
	// Convert the BlsSignature into the modified version that will aggregate
	// the public keys with the coefficient
	p.FinalSignature <- ModifiedBlsSignature(sig)

	return nil
}

// NewModifiedSubProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewModifiedSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewModifiedSubBlsCosi(n, vf, pairing.NewSuiteBn256())
}

// NewModifiedSubBlsCosi uses the default sub-protocol to make one compatible with
// the robust scheme
func NewModifiedSubBlsCosi(n *onet.TreeNodeInstance, vf protocol.VerificationFn, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
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

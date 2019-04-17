package asmsproto

import (
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/asmbls"
	"go.dedis.ch/kyber/v3/sign/cosi"
)

// ASMSignature is a signature that must be verified using coefficients
// derived from the public keys
type ASMSignature []byte

// GetMask returns the bytes representing the mask
func (sig ASMSignature) GetMask(suite pairing.Suite, pubkeys []kyber.Point) (*sign.Mask, error) {
	return protocol.BlsSignature(sig).GetMask(suite, pubkeys)
}

// Point returns the point associated with the signature
func (sig ASMSignature) Point(suite pairing.Suite) (kyber.Point, error) {
	return protocol.BlsSignature(sig).Point(suite)
}

// Verify returns an error if the signature can't be verified or nil if it matches
func (sig ASMSignature) Verify(suite pairing.Suite, msg []byte, pubkeys []kyber.Point) error {
	policy := cosi.NewThresholdPolicy(protocol.DefaultThreshold(len(pubkeys)))

	return sig.VerifyWithPolicy(suite, msg, pubkeys, policy)
}

// VerifyWithPolicy checks that the signature is correct and that the number of signers
// matches the policy
func (sig ASMSignature) VerifyWithPolicy(suite pairing.Suite, msg []byte, pubkeys []kyber.Point, policy cosi.Policy) error {
	lenCom := suite.G1().PointLen()
	if len(sig) < lenCom {
		return errors.New("invalid signature length")
	}

	signature := sig[:lenCom]

	// Unpack the participation mask and get the aggregate public key
	mask, err := sig.GetMask(suite, pubkeys)
	if err != nil {
		return err
	}

	aggPub, err := asmbls.AggregatePublicKeys(suite, mask)
	if err != nil {
		return err
	}

	err = asmbls.Verify(suite, aggPub, msg, signature)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}

	if !policy.Check(mask) {
		return fmt.Errorf("the policy is not fulfilled: %d", mask.CountEnabled())
	}

	return nil
}

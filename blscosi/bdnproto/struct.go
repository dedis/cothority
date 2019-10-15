package bdnproto

import (
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/kyber/v4/sign/bdn"
)

// BdnSignature is a signature that must be verified using coefficients
// derived from the public keys.
type BdnSignature []byte

// GetMask creates and returns the mask associated with the signature. If
// no mask has been appended, a mask with every bit enabled is returned.
func (sig BdnSignature) GetMask(suite pairing.Suite, pubkeys []kyber.Point) (*sign.Mask, error) {
	return protocol.BlsSignature(sig).GetMask(suite, pubkeys)
}

// Point returns the point associated with the signature.
func (sig BdnSignature) Point(suite pairing.Suite) (kyber.Point, error) {
	return protocol.BlsSignature(sig).Point(suite)
}

// Verify returns an error if the signature can't be verified or nil if it matches. A
// default threshold of f missing signatures with len(pubkeys) = 3f + 1 is assumed.
func (sig BdnSignature) Verify(suite pairing.Suite, msg []byte, pubkeys []kyber.Point) error {
	policy := sign.NewThresholdPolicy(protocol.DefaultThreshold(len(pubkeys)))

	return sig.VerifyWithPolicy(suite, msg, pubkeys, policy)
}

// VerifyWithPolicy checks that the signature is correct and that the number of signers
// matches the policy.
func (sig BdnSignature) VerifyWithPolicy(suite pairing.Suite, msg []byte, pubkeys []kyber.Point, policy sign.Policy) error {
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

	aggPub, err := bdn.AggregatePublicKeys(suite, mask)
	if err != nil {
		return err
	}

	err = bdn.Verify(suite, aggPub, msg, signature)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}

	if !policy.Check(mask) {
		return fmt.Errorf("the policy is not fulfilled: %d", mask.CountEnabled())
	}

	return nil
}

package protocol2

import (
	"fmt"

	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/bls2"
)

// ModifiedBlsSignature is a signature that must be verified using coefficients
// derived from the public keys
type ModifiedBlsSignature []byte

// GetMask returns the bytes representing the mask
func (sig ModifiedBlsSignature) GetMask(suite pairing.Suite, pubkeys []kyber.Point) (*bls.Mask, error) {
	return protocol.BlsSignature(sig).GetMask(suite, pubkeys)
}

// Verify returns an error if the signature can't be verified or nil if it matches
func (sig ModifiedBlsSignature) Verify(suite pairing.Suite, msg []byte, pubkeys []kyber.Point) error {
	lenCom := suite.G1().PointLen()
	signature := sig[:lenCom]

	// Unpack the participation mask and get the aggregate public key
	mask, err := sig.GetMask(suite, pubkeys)
	if err != nil {
		return err
	}

	aggPub, err := bls2.AggregatePublicKeys(suite, mask)
	if err != nil {
		return err
	}

	err = bls2.Verify(suite, aggPub, msg, signature)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}

	return nil
}

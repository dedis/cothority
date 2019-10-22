package cosuite

import (
	"hash"

	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/onet/v4/ciphersuite"
)

// CoSiCipherSuite is an extension of the Onet cipher suite interface that
// includes primitives to aggregate public keys and signatures that is
// necessary for collective signatures.
type CoSiCipherSuite interface {
	ciphersuite.CipherSuite

	// SignWithMask signs a message and set the mask of the signature so
	// that the signature is assigned to a conode. The resulting signature
	// can only be checked by aggregated public keys.
	SignWithMask(sk ciphersuite.SecretKey, msg []byte, mask *sign.Mask) (ciphersuite.Signature, error)

	// VerifyThreshold returns true if the aggregation has enough signatures.
	VerifyThreshold(ciphersuite.Signature, int) bool

	// AggregatePublicKeys makes the public key aggregation using the mask
	// of the signature to know which keys are include.
	AggregatePublicKeys([]ciphersuite.PublicKey, ciphersuite.Signature) (ciphersuite.PublicKey, error)

	// AggregateSignatures aggregates the list of signatures.
	AggregateSignatures([]ciphersuite.Signature, []ciphersuite.PublicKey) (ciphersuite.Signature, error)

	// Hash returns a hash context associated with the cipher suite.
	Hash() hash.Hash

	// Mask creates a mask of the public keys without any bit enabled.
	Mask([]ciphersuite.PublicKey) (*sign.Mask, error)

	// Count returns the number of single signatures contained in the one
	// passed in parameter.
	Count(ciphersuite.Signature) int
}

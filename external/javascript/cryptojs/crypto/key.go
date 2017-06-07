package crypto

import (
	"github.com/dedis/crypto/ed25519"
	"github.com/dedis/crypto/eddsa"
)

// Generate an EdDSA and return the marshal binary data
func KeyPairEdDSA() []byte {
	e := eddsa.NewEdDSA(nil)

	result, _ := e.MarshalBinary()
	return result
}

// Build a marshal binary of an EdDSA from a simple private key
func KeyPairFromPrivate(privateKey []byte) []byte {
	// Build the marshal binary without public key as we don't have it
	buf := make([]byte, 64)
	copy(buf[:32], privateKey)

	// Use it to instantiate a new EdDSA
	e := &eddsa.EdDSA{}
	e.UnmarshalBinary(buf)

	result, _ := e.MarshalBinary()
	return result
}

// Extract the public key from the marshal
func PublicKey(marshal []byte) []byte {
	return marshal[32:]
}

// Aggregate the given public keys in one single key
func AggregateKeys(keys [][]byte) []byte {
	suite := ed25519.NewAES128SHA256Ed25519(false)
	aggKey := suite.Point().Null()

	for _, k := range keys {
		public := suite.Point()
		public.UnmarshalBinary(k)

		aggKey.Add(aggKey, public)
	}

	result, _ := aggKey.MarshalBinary()
	return result
}

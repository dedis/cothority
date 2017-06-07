package crypto

import (
	"github.com/dedis/crypto/ed25519"
	"github.com/dedis/crypto/eddsa"
)

// Sign the given message with the provided EdDSA in marshal binary shape
func Sign(marshal, message []byte) []byte {
	// Extract the marshal binary
	e := eddsa.EdDSA{}
	e.UnmarshalBinary(marshal)

	signed, _ := e.Sign(message)
	return signed
}

// Verify a given signature with the given public key
// return a boolean which is true if the signature is verified
func Verify(pubkey, msg, signature []byte) bool {
	suite := ed25519.NewAES128SHA256Ed25519(false)

	public := suite.Point()
	public.UnmarshalBinary(pubkey)

	return eddsa.Verify(public, msg, signature) == nil
}

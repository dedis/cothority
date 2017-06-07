package main

import (
	"github.com/dedis/cothority/external/javascript/cryptojs/crypto"
	"github.com/gopherjs/gopherjs/js"
)

/**
 * Encapsulate the library in the cryptoJS object that you can
 * find the global JS object
 */
func main() {
	js.Global.Set("cryptoJS", map[string]interface{}{
		"keyPair":            crypto.KeyPairEdDSA,
		"keyPairFromPrivate": crypto.KeyPairFromPrivate,
		"publicKey":          crypto.PublicKey,
		"sign":               crypto.Sign,
		"verify":             crypto.Verify,
		"aggregateKeys":      crypto.AggregateKeys,
		"sha256":             crypto.Sha256,
		"sha512":             crypto.Sha512,
		"hashSkipBlock":      crypto.HashSkipBlock,
		"verifyForwardLink":  crypto.VerifyForwardLink,
	})
}

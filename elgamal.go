package onchain_secrets

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
)

// ElGamal holds the public ephemeral key K and the blinded message C.
type ElGamal struct {
	K abstract.Point
	C abstract.Point
}

// ElGamalEncrypt takes a group to encrypt under, the public key of the destination
// and the message. It returns an ElGamal structure and an eventual remainder,
// as the encrypted message-length is limited by group.Point().PickLen(), which
// is about 29 bytes for ed25519 curves.
func ElGamalEncrypt(group abstract.Group, pubkey abstract.Point, message []byte) (
	eg *ElGamal, remainder []byte) {

	// Embed the message (or as much of it as will fit) into a curve point.
	M, remainder := group.Point().Pick(message, random.Stream)
	max := group.Point().PickLen()
	if max > len(message) {
		max = len(message)
	}

	eg = &ElGamal{}
	// ElGamal-encrypt the point to produce ciphertext (K,C).
	k := group.Scalar().Pick(random.Stream) // ephemeral private key
	eg.K = group.Point().Mul(nil, k)        // ephemeral DH public key
	S := group.Point().Mul(pubkey, k)       // ephemeral DH shared secret
	eg.C = S.Add(S, M)                      // message blinded with secret
	return
}

// ElGamalDecrypt takes a group for the decryption, the private key of the destination
// and an ElGamal structure holding the public ephemeral key K and the blinded
// message C.
func ElGamalDecrypt(group abstract.Group, prikey abstract.Scalar, eg *ElGamal) (
	message []byte, err error) {

	// ElGamal-decrypt the ciphertext (K,C) to reproduce the message.
	S := group.Point().Mul(eg.K, prikey) // regenerate shared secret
	M := group.Point().Sub(eg.C, S)      // use to un-blind the message
	message, err = M.Data()              // extract the embedded data
	return
}

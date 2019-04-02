package pki

import (
	"bytes"

	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/onet/v3/network"
)

// PkProof is the proof of possession of a key
type PkProof struct {
	Public    []byte
	Nonce     []byte
	Signature []byte
}

// PkProofs is a list of PkProof
type PkProofs []PkProof

// Verify returns true if the service identity can be verified
func (pp PkProofs) Verify(srvid *network.ServiceIdentity) bool {
	pub, err := srvid.Public.MarshalBinary()
	if err != nil {
		return false
	}

	for _, p := range pp {
		if bytes.Equal(p.Public, pub) {
			msg := append(p.Public, p.Nonce...)
			if bls.Verify(pairingSuite, srvid.Public, msg, p.Signature) == nil {
				return true
			}
		}
	}

	return false
}

// RequestPkProof is the message for asking a proof
type RequestPkProof struct {
	// We use a nonce from the requester and the receiver to prevent
	// forged proof of possession
	Nonce []byte
}

// ResponsePkProof contains the response of a request
type ResponsePkProof struct {
	Proofs PkProofs
}

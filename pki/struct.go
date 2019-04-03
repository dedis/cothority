package pki

import (
	"bytes"
	"errors"
	"fmt"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3/network"
)

// SignFunc generates the signature of a message given the secret key
type SignFunc func(secret kyber.Scalar, msg []byte) ([]byte, error)

// VerifyFunc verifies the signature of a message given the public key
type VerifyFunc func(pub kyber.Point, msg []byte, sig []byte) error

// PkProof is the proof of possession of a key
type PkProof struct {
	Public    []byte
	Nonce     []byte
	Signature []byte
}

// PkProofs is a list of PkProof
type PkProofs []PkProof

// Verify returns true if the service identity can be verified
func (pp PkProofs) Verify(srvid *network.ServiceIdentity) error {
	pub, err := srvid.Public.MarshalBinary()
	if err != nil {
		return fmt.Errorf("couldn't marshal the public key: %v", err)
	}

	for _, p := range pp {
		if bytes.Equal(p.Public, pub) {
			if len(p.Nonce) != nonceLength {
				return errors.New("nonce length does not match")
			}

			f := verifyRegister[srvid.Suite]
			if f == nil {
				return errors.New("unknown suite used for the service")
			}

			msg := append(p.Public, p.Nonce...)
			err = f(srvid.Public, msg, p.Signature)
			if err != nil {
				return fmt.Errorf("signature verification failed: %v", err)
			}

			return nil
		}
	}

	return errors.New("couldn't find a proof")
}

// RequestPkProof is the message for asking a proof
type RequestPkProof struct{}

// ResponsePkProof contains the response of a request
type ResponsePkProof struct {
	Proofs PkProofs
}

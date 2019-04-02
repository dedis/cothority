// Package pki provides a Public Key Infrastructure that has the purpose of storing and
// getting proof of possession of the public keys a conode is holding for certain services
package pki

import (
	"crypto/rand"
	"fmt"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	bbolt "go.etcd.io/bbolt"
)

var pairingSuite = pairing.NewSuiteBn256()

// ServiceName is the name used to register the service
const ServiceName = "PKI"

const nonceLength = 64

func init() {
	onet.RegisterNewService(ServiceName, newPKIService)
}

// Service is the data structure of the service
type Service struct {
	*onet.ServiceProcessor

	db     *bbolt.DB
	bucket []byte
}

// GetProof returns the proofs of possession of a distant conode using either a local proof
// or by requesting it to the conode
func (s *Service) GetProof(srvid *network.ServerIdentity) ([]PkProof, error) {
	client := NewClient()
	proofs, err := client.GetProof(srvid)

	// TODO: database storage

	return proofs, err
}

// RequestProof returns the list of public-key proofs for the given server identity
func (s *Service) RequestProof(req *RequestPkProof) (*ResponsePkProof, error) {
	if len(req.Nonce) != 64 {
		return nil, fmt.Errorf("nonce needs to be of length %d", nonceLength)
	}

	proofs := make([]PkProof, len(s.ServerIdentity().ServiceIdentities))

	for i, k := range s.ServerIdentity().ServiceIdentities {
		if k.Suite == "bn256.adapter" {
			pub, err := k.Public.MarshalBinary()
			if err != nil {
				return nil, err
			}

			// need a nonce with enough entropy to prevent a brute force attack
			nonce, err := makeNonce()
			if err != nil {
				return nil, err
			}

			// combination of two nonces so that neither of the participants
			// can produce a forged proof of possession
			nonce = append(req.Nonce, nonce...)

			sig, err := bls.Sign(pairingSuite, k.GetPrivate(), append(pub, nonce...))
			if err != nil {
				return nil, err
			}

			proofs[i].Public = pub
			proofs[i].Nonce = nonce
			proofs[i].Signature = sig
		}
	}

	return &ResponsePkProof{Proofs: proofs}, nil
}

func newPKIService(c *onet.Context) (onet.Service, error) {
	db, bucket := c.GetAdditionalBucket([]byte("pki-service"))

	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		db:               db,
		bucket:           bucket,
	}

	err := service.RegisterHandlers(service.RequestProof)

	return service, err
}

func makeNonce() ([]byte, error) {
	// need a nonce with enough entropy to prevent a brute force attack
	nonce := make([]byte, nonceLength)
	_, err := rand.Read(nonce)

	return nonce, err
}

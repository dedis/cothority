// Package pki provides a Public Key Infrastructure that has the purpose of storing and
// getting proof of possession of the public keys that a conode is holding for certain services
package pki

import (
	"crypto/rand"
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	bbolt "go.etcd.io/bbolt"
)

// ServiceName is the name used to register the service
const ServiceName = "PKI"

const nonceLength = 8

var bn256Suite = pairing.NewSuiteBn256()
var ed25519Suite = suites.MustFind("Ed25519")

var signRegister map[string]SignFunc
var verifyRegister map[string]VerifyFunc

func init() {
	onet.RegisterNewService(ServiceName, newPKIService)

	// register for the signing functions
	signRegister = make(map[string]SignFunc)
	signRegister[bn256Suite.String()] = func(secret kyber.Scalar, msg []byte) ([]byte, error) {
		return bls.Sign(bn256Suite, secret, msg)
	}
	signRegister[cothority.Suite.String()] = func(secret kyber.Scalar, msg []byte) ([]byte, error) {
		return schnorr.Sign(cothority.Suite, secret, msg)
	}

	// register for the verification functions
	verifyRegister = make(map[string]VerifyFunc)
	verifyRegister[bn256Suite.String()] = func(pub kyber.Point, msg []byte, sig []byte) error {
		return bls.Verify(bn256Suite, pub, msg, sig)
	}
	verifyRegister[ed25519Suite.String()] = func(pub kyber.Point, msg []byte, sig []byte) error {
		return schnorr.Verify(ed25519Suite, pub, msg, sig)
	}
}

// Service is the data structure of the service
type Service struct {
	*onet.ServiceProcessor

	db     *bbolt.DB
	bucket []byte
	client *Client
}

// GetProof returns the proofs of possession of a distant conode using either a local proof
// or by requesting it to the conode
func (s *Service) GetProof(srvid *network.ServerIdentity) ([]PkProof, error) {
	proofs, ok, err := s.readProofs(srvid)
	if err != nil {
		return nil, err
	}
	if ok {
		return proofs, nil
	}

	proofs, err = s.client.GetProof(srvid)
	if err != nil {
		return nil, err
	}

	// store the proofs as we can reuse them later on
	// Note that proofs have been verified by the client already
	err = s.storeProofs(proofs)

	return proofs, err
}

// VerifyRoster takes a roster and make sure any server identity inside is honest
func (s *Service) VerifyRoster(roster *onet.Roster) error {
	for _, si := range roster.List {
		_, err := s.GetProof(si)
		if err != nil {
			return fmt.Errorf("couldn't verify %v: %v", si, err)
		}
	}

	return nil
}

// RequestProof returns the list of public-key proofs for the given server identity
func (s *Service) RequestProof(req *RequestPkProof) (*ResponsePkProof, error) {
	proofs := make([]PkProof, len(s.ServerIdentity().ServiceIdentities))

	for i, k := range s.ServerIdentity().ServiceIdentities {
		f := signRegister[k.Suite]
		if f == nil {
			return nil, errors.New("unknown suite for the service key pair")
		}

		pub, err := k.Public.MarshalBinary()
		if err != nil {
			return nil, err
		}

		// need a nonce to avoid a signature of the raw public key
		// that could be used elsewhere
		nonce, err := makeNonce()
		if err != nil {
			return nil, err
		}

		// Here it is important to use the public key because an attacker could forge
		// a proof of possession if the message signed doesn't have enough entropy using
		// H(m)^s / H(m)^t = H(m)^(s-t) where s is the attacker secret and t the honest
		// peer secret. Using the public key prevents the attacker to forge m.
		// Note: this applies mainly to the BLS scheme
		sig, err := f(k.GetPrivate(), append(pub, nonce...))
		if err != nil {
			return nil, err
		}

		proofs[i].Public = pub
		proofs[i].Nonce = nonce
		proofs[i].Signature = sig
	}

	return &ResponsePkProof{Proofs: proofs}, nil
}

// readProofs tries to get the proofs from the cache and return false if at least
// one is missing meaning a request will be necessary
func (s *Service) readProofs(si *network.ServerIdentity) ([]PkProof, bool, error) {
	proofs := []PkProof{}
	ok := true
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(s.bucket)

		for _, srvid := range si.ServiceIdentities {
			pub, err := srvid.Public.MarshalBinary()
			if err != nil {
				return err
			}

			buf := b.Get(pub)
			if buf == nil {
				ok = false
				return nil
			}

			var pr PkProof
			err = protobuf.Decode(buf, &pr)
			if err != nil {
				return err
			}

			proofs = append(proofs, pr)
		}

		return nil
	})

	return proofs, ok, err
}

// storeProofs stores the given proofs in the db
func (s *Service) storeProofs(proofs []PkProof) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(s.bucket)

		for _, pr := range proofs {
			buf, err := protobuf.Encode(&pr)
			if err != nil {
				return err
			}

			err = b.Put(pr.Public, buf)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func newPKIService(c *onet.Context) (onet.Service, error) {
	db, bucket := c.GetAdditionalBucket([]byte("pki-service"))

	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		db:               db,
		bucket:           bucket,
		client:           NewClient(),
	}

	err := service.RegisterHandlers(service.RequestProof)

	return service, err
}

func makeNonce() ([]byte, error) {
	nonce := make([]byte, nonceLength)
	_, err := rand.Read(nonce)

	return nonce, err
}

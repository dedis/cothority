package authprox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/sign/dss"
	"go.dedis.ch/kyber/v4/suites"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
	"go.dedis.ch/protobuf"
	bbolt "go.etcd.io/bbolt"
)

// ServiceName is the name of the Authentication Proxy service.
const ServiceName = "AuthProx"

var authProxID onet.ServiceID

type service struct {
	*onet.ServiceProcessor
	ctx        context.Context
	validators map[string]Validator
	db         *bbolt.DB
	bucket     []byte
}

func init() {
	var err error
	authProxID, err = onet.RegisterNewService(ServiceName, newService)
	if err != nil {
		log.ErrFatal(err, "could not register")
	}
	network.RegisterMessages(
		&EnrollRequest{}, &EnrollResponse{},
		&SignatureRequest{}, &SignatureResponse{},
		&EnrollmentsRequest{}, &EnrollmentsResponse{}, &EnrollmentInfo{},
		&ti{}, &dssConfig{},
	)
}

// A Validator is able to check the provided authInfo with respect to
// the thrid-party authentication system. Extracts the user-id and the
// (optional) hash of the message from the auth info.
type Validator interface {
	FindClaim(issuer string, authInfo []byte) (claim string, hash string, err error)
}

func (s *service) registerValidator(t string, v Validator) {
	if _, ok := s.validators[t]; ok {
		panic("cannot re-register a validator")
	}
	s.validators[t] = v
}

func (s *service) validator(in string) (Validator, error) {
	v, ok := s.validators[in]
	if !ok {
		return nil, fmt.Errorf("unknown auth type %v", in)
	}
	return v, nil
}

// ti is a struct to encode/decode a pair of type and issuer.
type ti struct {
	T, I string
}

type dssConfig struct {
	Participants []kyber.Point
	LongPri      share.PriShare
	LongPubs     []kyber.Point
}

func (s *service) find(typ, issuer string) (*dssConfig, error) {
	k, err := protobuf.Encode(&ti{T: typ, I: issuer})
	if err != nil {
		return nil, err
	}
	var buf []byte
	err = s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(s.bucket)
		if b == nil {
			return errors.New("nil bucket")
		}
		v := b.Get(k)
		if v == nil {
			return errors.New("claim not found")
		}
		// make a copy before leaving the tx
		buf = append(buf, v...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var out dssConfig
	err = protobuf.DecodeWithConstructors(buf, &out, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// Enrollments returns a list of enrollments. The result list is filtered by
// the Types and Issuers lists in the request. Empty filter lists are treated as
// "match all". The result is the AND or Types and Issers, and within one list,
// an enrollment is returned if any of the list items match.
func (s *service) Enrollments(req *EnrollmentsRequest) (*EnrollmentsResponse, error) {
	var resp EnrollmentsResponse
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(s.bucket)
		if b == nil {
			return errors.New("nil bucket")
		}
		// iterate through all the enrollments
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// Make copies, since protobuf.Decode might store slices of the data
			// from BoltDB for access after the txn.
			var k2 []byte
			k2 = append(k2, k...)
			var v2 []byte
			v2 = append(v2, v...)

			// Decode type, issuer
			var ti0 ti
			err := protobuf.Decode(k2, &ti0)
			if err != nil {
				return err
			}

			// Decode public key
			var out dssConfig
			err = protobuf.DecodeWithConstructors(v2, &out, network.DefaultConstructors(cothority.Suite))
			if err != nil {
				return err
			}

			// Filter by Types, if given.
			if len(req.Types) != 0 {
				var ok bool
				for _, t := range req.Types {
					if ti0.T == t {
						ok = true
						break
					}
				}
				if !ok {
					continue
				}
			}

			// Filter by Issuers, if given.
			if len(req.Issuers) != 0 {
				var ok bool
				for _, i := range req.Issuers {
					if ti0.I == i {
						ok = true
						break
					}
				}
				if !ok {
					continue
				}
			}

			resp.Enrollments = append(resp.Enrollments, EnrollmentInfo{
				Type:   ti0.T,
				Issuer: ti0.I,
				Public: out.LongPubs[0],
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Enroll will save the proposed secret key share (and associated information)
// in the local database.
func (s *service) Enroll(req *EnrollRequest) (*EnrollResponse, error) {
	// TODO: How is auth for the right to enroll done?
	// idea: allow other services (i.e. evoting) to define certain type/claims
	// as admins, require AuthInfo in EnrollmentRequest, and if the validated
	// claim is in the admin list, allow.

	// Prepare the stuff to be written
	k, err := protobuf.Encode(&ti{T: req.Type, I: req.Issuer})
	if err != nil {
		return nil, err
	}
	lpri := share.PriShare{
		I: req.LongPri.I,
		V: req.LongPri.V,
	}
	v, err := protobuf.Encode(&dssConfig{
		Participants: req.Participants,
		LongPri:      lpri,
		LongPubs:     req.LongPubs,
	})
	if err != nil {
		return nil, err
	}

	// Write the type/claim -> dssConfg into the database.
	err = s.db.Update(func(tx *bbolt.Tx) error {
		// Need to do the find inside of the Update tx, or else it is racy
		// with respect to other writers.
		b := tx.Bucket(s.bucket)
		if b == nil {
			return errors.New("nil bucket")
		}
		if ret := b.Get(k); ret != nil {
			return fmt.Errorf("enrollment already exists for type:issuer %v:%v", req.Type, req.Issuer)
		}
		return b.Put(k, v)
	})
	if err != nil {
		return nil, err
	}

	return &EnrollResponse{}, nil
}

// Signature will verify the authentication information in
// the request, according to the rules specific to that authentication type.
// If the information is valid, it will then generate a partial signature
// on a new message which binds together the claim found from the authentication
// information, and the message in the request, using the secret key share associated
// with the type and issuer. It is the caller's responsibility to gather a
// threshold of key shares and then combine them into a final signature.
func (s *service) Signature(req *SignatureRequest) (*SignatureResponse, error) {
	if req == nil {
		return nil, errors.New("no request")
	}

	// Look for a validator for the external type requested.
	validator, err := s.validator(req.Type)
	if err != nil {
		return nil, err
	}

	// Use the validator to extract a claim from the auth info.
	claim, hashStr, err := validator.FindClaim(req.Issuer, req.AuthInfo)
	if err != nil {
		return nil, err
	}

	// TODO: For now, hash will always be unset because the only production
	// validator is OIDC, and we have not yet figured out how to tunnel the
	// hash of the transaction through OpenID. The "nonce" claim seems like
	// the right tool for the job, but the oauth2 package does not let
	// us set it, and dex does not let refreshed tokens update it.
	if hashStr != "" {
		// Because it travels through the 3rd party auth system, the hash
		// is a hex encoded string here.
		hashBin, err := hex.DecodeString(hashStr)
		if err != nil {
			return nil, fmt.Errorf("unable to decode hash %v: %v", hashStr, err)
		}

		// We managed to extract the hash from the authInfo, so we need to make sure
		// that H(req.Message) == hash, so that we know this is not a replay attempt.
		h := sha256.Sum256(req.Message)
		if !bytes.Equal(h[:], hashBin) {
			return nil, errors.New("hash from AuthInfo does not match hash of message")
		}
	}

	// Given this claim, sign a
	h := sha256.New()
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(len(claim)))
	h.Write(b)
	h.Write([]byte(claim))
	h.Write(req.Message)
	msg2 := h.Sum(nil)

	// Find the config that is associated with the issuer.
	dsscfg, err := s.find(req.Type, req.Issuer)
	if err != nil {
		return nil, fmt.Errorf("cannot find key: %v", err)
	}

	// Make the signature.
	suite := suites.MustFind("ed25519")
	rpi := share.PriShare{
		I: req.RandPri.I,
		V: req.RandPri.V,
	}
	priv := s.ServerIdentity().GetPrivate()
	if priv == nil {
		return nil, errors.New("server has no private key")
	}
	d, err := dss.NewDSS(suite, priv,
		dsscfg.Participants,
		&dks{dsscfg.LongPri, dsscfg.LongPubs},
		&dks{rpi, req.RandPubs},
		msg2, threshold(len(dsscfg.Participants)))
	if err != nil {
		return nil, err
	}
	ps, err := d.PartialSig()
	if err != nil {
		return nil, err
	}

	ps1 := PartialSig{
		Partial: PriShare{
			I: ps.Partial.I,
			V: ps.Partial.V,
		},
		SessionID: ps.SessionID,
		Signature: ps.Signature,
	}
	return &SignatureResponse{PartialSignature: ps1}, nil
}

func newService(c *onet.Context) (onet.Service, error) {
	db, bucket := c.GetAdditionalBucket([]byte("authproxy-claims"))
	s := &service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		ctx:              context.Background(),
		validators:       make(map[string]Validator),
		db:               db,
		bucket:           bucket,
	}
	if err := s.RegisterHandlers(
		s.Enroll,
		s.Signature,
		s.Enrollments,
	); err != nil {
		log.ErrFatal(err, "Could not register handlers.")
	}

	// Register validators here
	s.registerValidator("oidc", &oidcValidator{})

	return s, nil
}

type dks struct {
	pri  share.PriShare
	pubs []kyber.Point
}

// check that dkg correctly implements dss.DistKeyShare;
// this is a wrapper we use to work around the fact that package
// dss expects to take input from a DKG, but we fake it.
var _ dss.DistKeyShare = (*dks)(nil)

func (d *dks) PriShare() *share.PriShare  { return &d.pri }
func (d *dks) Commitments() []kyber.Point { return d.pubs }

func faultThreshold(n int) int {
	return (n - 1) / 3
}

func threshold(n int) int {
	return n - faultThreshold(n)
}

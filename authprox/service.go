package authprox

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/sign/dss"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// ServiceName is the name of the Authentication Proxy service.
const ServiceName = "AuthProx"

var authProxID onet.ServiceID

type service struct {
	*onet.ServiceProcessor
	ctx        context.Context
	validators map[string]Validator
	db         *bolt.DB
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
// the thrid-party authentication system. Return the user-id and the extra_data
// associated with this auth info, or error.
type Validator interface {
	FindClaim(issuer string, authInfo []byte) (claim string, extraData string, err error)
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
	Secret       kyber.Scalar
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
	err = s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(s.bucket).Get(k)
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

func (s *service) Enrollments(req *EnrollmentsRequest) (*EnrollmentsResponse, error) {
	var resp EnrollmentsResponse
	s.db.View(func(tx *bolt.Tx) error {
		// iterate through all the enrollments
		c := tx.Bucket(s.bucket).Cursor()
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
	return &resp, nil
}

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
		Secret:       req.Secret,
		Participants: req.Participants,
		LongPri:      lpri,
		LongPubs:     req.LongPubs,
	})
	if err != nil {
		return nil, err
	}

	// Write the type/claim -> dssConfg into the database.
	err = s.db.Update(func(tx *bolt.Tx) error {
		// Need to do the find inside of the Update tx, or else it is racy
		// with respect to other writers.
		if ret := tx.Bucket(s.bucket).Get(k); ret != nil {
			return fmt.Errorf("enrollment already exists for type:issuer %v:%v", req.Type, req.Issuer)
		}
		return tx.Bucket(s.bucket).Put(k, v)
	})
	if err != nil {
		return nil, err
	}

	return &EnrollResponse{}, nil
}

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
	claim, _, err := validator.FindClaim(req.Issuer, req.AuthInfo)
	if err != nil {
		return nil, err
	}

	// TODO: check that if extraData != "", then extraData == msg

	// Given this claim, sign a
	h := sha256.New()
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(len(claim)))
	h.Write(b)
	h.Write([]byte(claim))
	h.Write(req.Message)
	msg2 := h.Sum(nil)

	// Find the config that is associated the issuer.
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
	d, err := dss.NewDSS(suite, dsscfg.Secret,
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

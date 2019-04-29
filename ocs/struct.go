package ocs

import (
	"crypto/sha256"
	"errors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/kyber/v3"
)

func (op OCSProof) Verify() error {
	if len(op.Signatures) != len(op.Roster.List) {
		return errors.New("length of signatures is not equal to roster list length")
	}
	msg, err := op.Message()
	if err != nil {
		return Erret(err)
	}
	for i, si := range op.Roster.List {
		err := schnorr.Verify(cothority.Suite, si.ServicePublic(ServiceName), msg, op.Signatures[i])
		if err != nil {
			return Erret(err)
		}
	}
	return nil
}

func (op OCSProof) Message() ([]byte, error) {
	hash := sha256.New()
	hash.Write(op.OcsID)
	coc := CreateOCS{
		Roster:          op.Roster,
		PolicyReencrypt: op.PolicyReencrypt,
		PolicyReshare:   op.PolicyReshare,
	}
	buf, err := protobuf.Encode(&coc)
	if err != nil {
		return nil, Erret(err)
	}
	hash.Write(buf)
	return hash.Sum(nil), nil
}

func (ar AuthReencrypt) Xc() (kyber.Point, error) {
	if ar.X509Cert != nil {
		return GetPointFromCert(ar.X509Cert.Certificates[0], OIDEphemeralKey)
	}
	if ar.ByzCoin != nil {
		return nil, errors.New("can't get ephemeral key from ByzCoin yet")
	}
	return nil, errors.New("need to have authentication for X509 or ByzCoin")
}

func (ar AuthReencrypt) U() (kyber.Point, error) {
	if ar.X509Cert != nil {
		return ar.X509Cert.U, nil
	}
	if ar.ByzCoin != nil {
		return nil, errors.New("can't get secret from ByzCoin yet")
	}
	return nil, errors.New("need to have authentication for X509 or ByzCoin")
}

func NewOCSID(X kyber.Point) (OCSID, error) {
	return X.MarshalBinary()
}

func (ocs OCSID) X() (kyber.Point, error) {
	X := cothority.Suite.Point()
	err := Erret(X.UnmarshalBinary(ocs))
	return X, err
}

package ocs

import (
	"crypto/sha256"
	"crypto/x509"
	"errors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/kyber/v3"

	"go.dedis.ch/onet/v3"
)

func (ocs CreateOCS) verify() error {
	if err := ocs.PolicyReencrypt.verify(ocs.Roster); err != nil {
		return err
	}
	if err := ocs.PolicyReshare.verify(ocs.Roster); err != nil {
		return err
	}
	return nil
}

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

func (re Reshare) verify() error {
	return errors.New("not yet implemented")
}

func (p Policy) verify(r onet.Roster) error {
	if p.X509Cert != nil {
		return p.X509Cert.verify(r)
	}
	if p.ByzCoin != nil {
		return p.ByzCoin.verify(r)
	}
	return errors.New("need to have a policy for X509 or ByzCoin")
}

func (px PolicyX509Cert) verify(r onet.Roster) error {
	// TODO: decide how to make sure the policy fits the reencryption / resharing
	return nil
}

func (px PolicyByzCoin) verify(r onet.Roster) error {
	return Erret(errors.New("not yet implemented"))
}

func (ar AuthReencrypt) verify(p Policy, X, U kyber.Point) error {
	if ar.X509Cert == nil || p.X509Cert == nil {
		return errors.New("currently only checking X509 policies")
	}
	root, err := x509.ParseCertificate(p.X509Cert.CA[0])
	if err != nil {
		return Erret(err)
	}
	auth, err := x509.ParseCertificate(ar.X509Cert.Certificates[0])
	if err != nil {
		return Erret(err)
	}

	ocsID, err := NewOCSID(X)
	if err != nil {
		return Erret(err)
	}
	return Erret(Verify(root, auth, ocsID, U))
}

func (ar AuthReencrypt) Xc() (kyber.Point, error) {
	if ar.X509Cert != nil {
		return getPointFromCert(ar.X509Cert.Certificates[0], EphemeralKeyOID)
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

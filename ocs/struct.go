package ocs

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"runtime"
	"strings"

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

func (ocs CreateOCS) CheckOCSSignature(sig []byte, X OCSID) error {
	// TODO: test signature
	return nil
	if sig == nil {
		return errors.New("no signature given")
	}
	hash := sha256.New()
	X.MarshalTo(hash)
	buf, err := protobuf.Encode(ocs)
	if err != nil {
		return erret(err)
	}
	hash.Write(buf)
	return erret(schnorr.Verify(cothority.Suite, ocs.Roster.Aggregate, hash.Sum(nil), sig))
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
	return erret(errors.New("net yet implemented"))
}

func (ar AuthReencrypt) verify(p Policy, X, U kyber.Point) error {
	if ar.X509Cert == nil || p.X509Cert == nil {
		return errors.New("currently only checking X509 policies")
	}
	root, err := x509.ParseCertificate(p.X509Cert.CA[0])
	if err != nil {
		return erret(err)
	}
	auth, err := x509.ParseCertificate(ar.X509Cert.Certificates[0])
	if err != nil {
		return erret(err)
	}
	wid, err := getExtensionFromCert(auth, WriteIdOID)
	if err != nil {
		return erret(err)
	}
	err = WriteID(wid).Verify(X, U)
	if err != nil {
		return erret(err)
	}

	return erret(Verify(root, auth))
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

type WriteID []byte

func NewWriteID(X, U kyber.Point) (WriteID, error) {
	wid := sha256.New()
	_, err := X.MarshalTo(wid)
	if err != nil {
		return nil, erret(err)
	}
	_, err = U.MarshalTo(wid)
	if err != nil {
		return nil, erret(err)
	}
	return wid.Sum(nil), nil
}

func (wid WriteID) Verify(X, U kyber.Point) error {
	other, err := NewWriteID(X, U)
	if err != nil {
		return erret(err)
	}
	if bytes.Compare(wid, other) != 0 {
		return errors.New("not the same writeID")
	}
	return nil
}

func getPointFromCert(certBuf []byte, extID asn1.ObjectIdentifier) (kyber.Point, error) {
	cert, err := x509.ParseCertificate(certBuf)
	if err != nil {
		return nil, erret(err)
	}
	secret := cothority.Suite.Point()
	secretBuf, err := getExtensionFromCert(cert, extID)
	if err != nil {
		return nil, erret(err)
	}
	err = secret.UnmarshalBinary(secretBuf)
	return secret, erret(err)
}

func getExtensionFromCert(cert *x509.Certificate, extID asn1.ObjectIdentifier) ([]byte, error) {
	var buf []byte
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(extID) {
			buf = ext.Value
			break
		}
	}
	if buf == nil {
		return nil, errors.New("didn't find extension in certificate")
	}
	return buf, nil
}

func erret(err error) error {
	if err == nil {
		return nil
	}
	pc, _, line, _ := runtime.Caller(1)
	errStr := err.Error()
	if strings.HasPrefix(errStr, "erret") {
		errStr = "\n\t" + errStr
	}
	return fmt.Errorf("erret at %s: %d -> %s", runtime.FuncForPC(pc).Name(), line, errStr)
}

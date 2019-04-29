package ocs

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"strings"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/onet/v3"

	"go.dedis.ch/kyber/v3"
)

func (cocs CreateOCS) verifyAuth(policies []PolicyCreate) error {
	for _, p := range policies {
		if err := p.verify(cocs.Auth, cocs.PolicyReencrypt, cocs.PolicyReshare, cocs.Roster); err == nil {
			return nil
		}
	}
	return errors.New("no policy matches against the authorization")
}

func (pc PolicyCreate) verify(auth AuthCreate, pRC PolicyReencrypt, pRS PolicyReshare, roster onet.Roster) error {
	if pc.X509Cert != nil && auth.X509Cert != nil && pRC.X509Cert != nil && pRS.X509Cert != nil {
		return pc.X509Cert.verify(auth.X509Cert.Certificates, func(vo x509.VerifyOptions, cert *x509.Certificate) error {
			return VerifyCreate(vo, cert, pRC.X509Cert, pRS.X509Cert, roster)
		})
	}
	if pc.ByzCoin != nil && auth.ByzCoin != nil && pRC.ByzCoin != nil && pRS.ByzCoin != nil {
		return errors.New("byzcoin verification not implemented yet")
	}
	return errors.New("no matching policy/auth found")
}

func (pc PolicyReencrypt) verify(auth AuthReencrypt, X, U kyber.Point) error {
	if X == nil || U == nil {
		return errors.New("need both X and U for verification")
	}
	if pc.X509Cert != nil && auth.X509Cert != nil {
		return pc.X509Cert.verify(auth.X509Cert.Certificates, func(vo x509.VerifyOptions, cert *x509.Certificate) error {
			return VerifyReencrypt(vo, cert, X, U)
		})
	}
	if pc.ByzCoin != nil && auth.ByzCoin != nil {
		return errors.New("byzcoin verification not implemented yet")
	}
	return errors.New("no matching policy/auth found")
}

func (pc PolicyReshare) verify(auth AuthReshare, r onet.Roster) error {
	if len(r.List) < 2 {
		return errors.New("roster must have at least 2 nodes")
	}
	if pc.X509Cert != nil && auth.X509Cert != nil {
		return pc.X509Cert.verify(auth.X509Cert.Certificates, func(vo x509.VerifyOptions, cert *x509.Certificate) error {
			return VerifyReshare(vo, cert, r)
		})
	}
	if pc.ByzCoin != nil && auth.ByzCoin != nil {
		return errors.New("byzcoin verification not implemented yet")
	}
	return errors.New("no matching policy/auth found")
}

type verifyFunc func(vo x509.VerifyOptions, cert *x509.Certificate) error

func (p509 PolicyX509Cert) verify(certBufs [][]byte, vf verifyFunc) error {
	var certs []*x509.Certificate
	for _, certBuf := range certBufs {
		cert, err := x509.ParseCertificate(certBuf)
		if err != nil {
			return err
		}
		certs = append(certs, cert)
	}
	count := 0
	var errs []string
	for _, caBuf := range p509.CA {
		ca, err := x509.ParseCertificate(caBuf)
		if err != nil {
			return err
		}
		roots := x509.NewCertPool()
		roots.AddCert(ca)
		opt := x509.VerifyOptions{Roots: roots}
		for _, cert := range certs {
			if err := vf(opt, cert); err == nil {
				count++
				break
			} else {
				errs = append(errs, err.Error())
			}
		}
	}
	if count >= p509.Threshold {
		return nil
	}
	return errors.New("didn't reach threshold - errs: " + strings.Join(errs, "\n -- "))
}

var (
	// selection of OID numbers is not random See documents
	// https://tools.ietf.org/html/rfc5280#page-49
	// https://tools.ietf.org/html/rfc7229
	OIDWriteId         = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 1}
	OIDEphemeralKey    = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 2}
	OIDPolicyReencrypt = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 3}
	OIDPolicyReshare   = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 4}
	OIDRoster          = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 5}
)

// VerifyReencrypt takes a root certificate and the certificate to verify. It then verifies
// the certificates with regard to the signature of the root-certificate to the
// authCert.
// ocsID is the ID of the LTS cothority, while U is the commitment to the secret.
func VerifyReencrypt(vo x509.VerifyOptions, cert *x509.Certificate, X kyber.Point, U kyber.Point) (err error) {
	wid, err := GetExtensionFromCert(cert, OIDWriteId)
	if err != nil {
		return Erret(err)
	}
	err = WriteID(wid).Verify(X, U)
	if err != nil {
		return Erret(err)
	}

	unmarkUnhandledCriticalExtension(cert, OIDWriteId)
	unmarkUnhandledCriticalExtension(cert, OIDEphemeralKey)

	_, err = cert.Verify(vo)
	return Erret(err)
}

// VerifyCreate takes a root certificate and the certificate to verify. It then verifies
// the certificates with regard to the signature of the root-certificate to the
// authCert.
// ocsID is the ID of the LTS cothority, while U is the commitment to the secret.
func VerifyCreate(vo x509.VerifyOptions, cert *x509.Certificate, policyReencrypt, policyReshare *PolicyX509Cert,
	roster onet.Roster) (err error) {
	pRcBuf, err := GetExtensionFromCert(cert, OIDPolicyReencrypt)
	if err != nil {
		return Erret(err)
	}
	pBuf, err := protobuf.Encode(policyReencrypt)
	if err != nil {
		return Erret(err)
	}
	if bytes.Compare(pRcBuf, pBuf) != 0 {
		return errors.New("reencryption-policy doesn't match policy in certificate")
	}

	pRsBuf, err := GetExtensionFromCert(cert, OIDPolicyReshare)
	if err != nil {
		return Erret(err)
	}
	pBuf, err = protobuf.Encode(policyReshare)
	if err != nil {
		return Erret(err)
	}
	if bytes.Compare(pRsBuf, pBuf) != 0 {
		return errors.New("reshare-policy doesn't match policy in certificate")
	}

	rosterBuf, err := GetExtensionFromCert(cert, OIDRoster)
	if err != nil {
		return Erret(err)
	}
	rBuf, err := protobuf.Encode(&roster)
	if err != nil {
		return Erret(err)
	}
	if bytes.Compare(rosterBuf, rBuf) != 0 {
		return errors.New("roster doesn't match roster in certificate")
	}

	unmarkUnhandledCriticalExtension(cert, OIDPolicyReencrypt)
	unmarkUnhandledCriticalExtension(cert, OIDPolicyReshare)
	unmarkUnhandledCriticalExtension(cert, OIDRoster)

	_, err = cert.Verify(vo)
	return Erret(err)
}

func VerifyReshare(vo x509.VerifyOptions, cert *x509.Certificate, r onet.Roster) (err error) {
	return errors.New("reshare verification not yet implemented")
}

// WriteID is the ID that will be revealed to the X509 verification method.
type WriteID []byte

func NewWriteID(X, U kyber.Point) (WriteID, error) {
	if X == nil || U == nil {
		return nil, errors.New("X or U is missing")
	}
	wid := sha256.New()
	_, err := X.MarshalTo(wid)
	if err != nil {
		return nil, Erret(err)
	}
	_, err = U.MarshalTo(wid)
	if err != nil {
		return nil, Erret(err)
	}
	return wid.Sum(nil), nil
}

func (wid WriteID) Verify(X, U kyber.Point) error {
	other, err := NewWriteID(X, U)
	if err != nil {
		return Erret(err)
	}
	if bytes.Compare(wid, other) != 0 {
		return errors.New("not the same writeID")
	}
	return nil
}

func GetPointFromCert(certBuf []byte, extID asn1.ObjectIdentifier) (kyber.Point, error) {
	cert, err := x509.ParseCertificate(certBuf)
	if err != nil {
		return nil, Erret(err)
	}
	secret := cothority.Suite.Point()
	secretBuf, err := GetExtensionFromCert(cert, extID)
	if err != nil {
		return nil, Erret(err)
	}
	err = secret.UnmarshalBinary(secretBuf)
	return secret, Erret(err)
}

func GetExtensionFromCert(cert *x509.Certificate, extID asn1.ObjectIdentifier) ([]byte, error) {
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

func unmarkUnhandledCriticalExtension(cert *x509.Certificate, id asn1.ObjectIdentifier) {
	for i, extension := range cert.UnhandledCriticalExtensions {
		if id.Equal(extension) {
			cert.UnhandledCriticalExtensions = append(cert.UnhandledCriticalExtensions[0:i],
				cert.UnhandledCriticalExtensions[i+1:]...)
			return
		}
	}
}

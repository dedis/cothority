package ocs

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"

	"go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
)

var (
	// selection of OID numbers is not random See documents
	// https://tools.ietf.org/html/rfc5280#page-49
	// https://tools.ietf.org/html/rfc7229
	WriteIdOID      = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 1}
	EphemeralKeyOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 2}
)

// Verify takes a root certificate and the certificate to verify. It then verifies
// the certificates with regard to the signature of the root-certificate to the
// authCert.
// ocsID is the ID of the LTS cothority, while U is the commitment to the secret.
func Verify(rootCert *x509.Certificate, authCert *x509.Certificate, ocsID OCSID, U kyber.Point) (err error) {
	roots := x509.NewCertPool()
	roots.AddCert(rootCert)

	cert, err := x509.ParseCertificate(authCert.Raw)
	if err != nil {
		return Erret(err)
	}

	opts := x509.VerifyOptions{
		Roots: roots,
	}

	wid, err := getExtensionFromCert(authCert, WriteIdOID)
	if err != nil {
		return Erret(err)
	}
	X, err := ocsID.X()
	if err != nil {
		return Erret(err)
	}
	err = WriteID(wid).Verify(X, U)
	if err != nil {
		return Erret(err)
	}

	unmarkUnhandledCriticalExtension(cert, WriteIdOID)
	unmarkUnhandledCriticalExtension(cert, EphemeralKeyOID)

	_, err = cert.Verify(opts)
	return Erret(err)
}

// WriteID is the ID that will be revealed to the
type WriteID []byte

func NewWriteID(X, U kyber.Point) (WriteID, error) {
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

func NewOCSID(X kyber.Point) (OCSID, error) {
	return X.MarshalBinary()
}

func (ocs OCSID) X() (kyber.Point, error) {
	X := cothority.Suite.Point()
	err := Erret(X.UnmarshalBinary(ocs))
	return X, err
}

func (ocs OCSID) Verify(roster onet.Roster, policyReencrypt, policyReshare Policy, sig []byte) error {
	return Erret(errors.New("use CreateOCS.CheckOCSSignature"))
	msg := sha256.New()
	msg.Write(ocs)
	policyBuf, err := protobuf.Encode(policyReencrypt)
	if err != nil {
		return Erret(err)
	}
	msg.Write(policyBuf)
	policyBuf, err = protobuf.Encode(policyReencrypt)
	if err != nil {
		return Erret(err)
	}
	msg.Write(policyBuf)
	agg, err := roster.ServiceAggregate(calypso.ServiceName)
	if err != nil {
		return Erret(err)
	}
	return Erret(schnorr.Verify(cothority.Suite, agg, msg.Sum(nil), sig))
}

func getPointFromCert(certBuf []byte, extID asn1.ObjectIdentifier) (kyber.Point, error) {
	cert, err := x509.ParseCertificate(certBuf)
	if err != nil {
		return nil, Erret(err)
	}
	secret := cothority.Suite.Point()
	secretBuf, err := getExtensionFromCert(cert, extID)
	if err != nil {
		return nil, Erret(err)
	}
	err = secret.UnmarshalBinary(secretBuf)
	return secret, Erret(err)
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

func unmarkUnhandledCriticalExtension(cert *x509.Certificate, id asn1.ObjectIdentifier) {
	for i, extension := range cert.UnhandledCriticalExtensions {
		if id.Equal(extension) {
			cert.UnhandledCriticalExtensions = append(cert.UnhandledCriticalExtensions[0:i],
				cert.UnhandledCriticalExtensions[i+1:]...)
			return
		}
	}
}

func getExtension(certificate *x509.Certificate, id asn1.ObjectIdentifier) *pkix.Extension {

	for _, ext := range certificate.Extensions {
		if ext.Id.Equal(id) {
			return &ext
		}
	}

	return nil
}

// TODO: add CreateX509(rootCA, time, writeID, ephemeralKey)

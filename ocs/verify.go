package ocs

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
)

var (
	// selection of OID numbers is not random See documents
	// https://tools.ietf.org/html/rfc5280#page-49
	// https://tools.ietf.org/html/rfc7229
	WriteIdOID       = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 1}
	EphemeralKeyOID  = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 2}
	ElGamalCommitOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 13, 3}
)

func Verify(rootCert *x509.Certificate, toVerify *x509.Certificate) (err error) {
	roots := x509.NewCertPool()
	roots.AddCert(rootCert)

	cert, err := x509.ParseCertificate(toVerify.Raw)
	if err != nil {
		return erret(err)
	}

	opts := x509.VerifyOptions{
		Roots: roots,
	}

	unmarkUnhandledCriticalExtension(cert, ElGamalCommitOID)
	unmarkUnhandledCriticalExtension(cert, WriteIdOID)
	unmarkUnhandledCriticalExtension(cert, EphemeralKeyOID)

	_, err = cert.Verify(opts)
	return erret(err)
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

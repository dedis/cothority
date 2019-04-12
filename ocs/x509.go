package ocs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"

	"go.dedis.ch/kyber/v3"
)

// CreateCaCert is used for tests and returns a new private key, as well as a CA certificate.
func CreateCaCert() (caPrivKey *ecdsa.PrivateKey, cert *x509.Certificate, err error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(25 * 365 * 24 * time.Hour)
	serialNumber := big.NewInt(1)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "ByzGen signer org1",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
		IsCA:                  true,
	}
	caPrivKey, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, nil, Erret(err)
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, Erret(err)
	}

	cert, err = x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, Erret(err)
	}
	return
}

// CreateReencryptCert is used for tests and can create a certificate for one of the nodes.
func CreateReencryptCert(caCert *x509.Certificate, caPrivKey *ecdsa.PrivateKey,
	writeID []byte, ephemeralPublicKey kyber.Point) (*x509.Certificate, error) {

	notBefore := time.Now()
	notAfter := notBefore.Add(14 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, Erret(err)
	}

	writeIdExt := pkix.Extension{
		Id:       WriteIdOID,
		Critical: true,
		Value:    writeID,
	}

	ephBuf, err := ephemeralPublicKey.MarshalBinary()
	if err != nil {
		return nil, Erret(err)
	}
	ephemeralKeyExt := pkix.Extension{
		Id:       EphemeralKeyOID,
		Critical: true,
		Value:    ephBuf,
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "Ephemeral read operation & Co",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	template.ExtraExtensions = append(template.ExtraExtensions, writeIdExt, ephemeralKeyExt)

	throwaway, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, Erret(err)
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &throwaway.PublicKey, caPrivKey)
	if err != nil {
		return nil, Erret(err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, Erret(err)
	}

	return cert, nil
}

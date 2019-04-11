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

// openssl ecparam -name secp384r1 -genkey -noout -outform der -out secp384r1-key.der
// openssl pkcs8 -topk8 -nocrypt -outform der -inform der -in secp384r1-key.der -out secp384r1-pkcs8.der
// openssl ec -inform der -in secp384r1-key.der -pubout -outform der -out secp384r1-pub.der

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
		return nil, nil, erret(err)
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, erret(err)
	}

	cert, err = x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, erret(err)
	}
	return
}

func CreateReencryptCert(caCert *x509.Certificate, caPrivKey *ecdsa.PrivateKey,
	ocsID OCSID, elGamalCommit kyber.Point, ephemeralPublicKey kyber.Point) (*x509.Certificate, error) {

	notBefore := time.Now()
	notAfter := notBefore.Add(14 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, erret(err)
	}

	ocsBuf, err := ocsID.MarshalBinary()
	if err != nil {
		return nil, erret(err)
	}
	writeIdExt := pkix.Extension{
		Id:       WriteIdOID,
		Critical: true,
		Value:    ocsBuf,
	}

	ephBuf, err := ephemeralPublicKey.MarshalBinary()
	if err != nil {
		return nil, erret(err)
	}
	ephemeralKeyExt := pkix.Extension{
		Id:       EphemeralKeyOID,
		Critical: true,
		Value:    ephBuf,
	}

	elGaBuf, err := elGamalCommit.MarshalBinary()
	if err != nil {
		return nil, erret(err)
	}
	elGamalCommitExt := pkix.Extension{
		Id:       ElGamalCommitOID,
		Critical: true,
		Value:    elGaBuf,
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

	template.ExtraExtensions = append(template.ExtraExtensions, writeIdExt, ephemeralKeyExt, elGamalCommitExt)

	throwaway, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, erret(err)
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &throwaway.PublicKey, caPrivKey)
	if err != nil {
		return nil, erret(err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, erret(err)
	}

	return cert, nil
}

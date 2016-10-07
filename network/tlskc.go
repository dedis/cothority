package network

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"

	"github.com/dedis/cothority/log"
)

// TLSKC is a TLS Key Cert storage with methods to return correct server-
// and client-configurations.
type TLSKC struct {
	// PEM-encoded certificate.
	Cert []byte
	// PEM-encoded key - can be empty for a remote server where we don't
	// have the key.
	Key []byte
}

// NewTLSCert returns a x509-certificate valid for all CommonNames.
func NewTLSCert(serial *big.Int, country, org, orgUnit string,
	validYear int, subjectKeyID []byte) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Country:            []string{country},
			Organization:       []string{org},
			OrganizationalUnit: []string{orgUnit},
			CommonName:         "*",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(validYear, 0, 0),
		SubjectKeyId:          subjectKeyID,
		BasicConstraintsValid: true,
		IsCA:        true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign |
			x509.KeyUsageKeyAgreement,
	}
}

// NewTLSKC returns a TLSKC-structure with a PEM-encoded certificate and key.
func NewTLSKC(ca *x509.Certificate, keyLen int) (*TLSKC, error) {
	tlskc := &TLSKC{}
	if keyLen < 2048 {
		log.Warn("Small key-length:", keyLen)
	}
	priv, _ := rsa.GenerateKey(rand.Reader, keyLen)
	pub := &priv.PublicKey
	cert, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		return nil, err
	}
	tlskc.Cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE",
		Bytes: cert})
	tlskc.Key = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return tlskc, nil
}

// ConfigClient returns a tls.Config usable for a call to tls.Dial.
func (t *TLSKC) ConfigClient() (*tls.Config, error) {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(t.Cert)
	if !ok {
		return nil, errors.New("Couldn't get root cert.")
	}
	return &tls.Config{RootCAs: roots, InsecureSkipVerify: false}, nil
}

// ConfigServer returns a tls.Config usable for a call to tls.Listen.
func (t *TLSKC) ConfigServer() (*tls.Config, error) {
	cert, err := tls.X509KeyPair(t.Cert, t.Key)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

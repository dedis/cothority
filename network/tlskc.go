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

	"net"

	"github.com/dedis/cothority/log"
)

type TLSCertPEM []byte
type TLSKeyPEM []byte

// NewTLSCert returns a x509-certificate valid for all CommonNames.
func NewTLSCert(serial *big.Int, country, org, orgUnit string,
	validYear int, subjectKeyID []byte,
	ips []net.IP) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Country:            []string{country},
			Organization:       []string{org},
			OrganizationalUnit: []string{orgUnit},
			//CommonName:         "*",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(validYear, 0, 0),
		SubjectKeyId:          subjectKeyID,
		BasicConstraintsValid: true,
		IsCA:        true,
		IPAddresses: ips,
		DNSNames:    []string{"localhost"},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign |
			x509.KeyUsageKeyAgreement,
	}
}

// NewTLSKC returns a TLSKC-structure with a PEM-encoded certificate and key.
func NewCertKey(ca *x509.Certificate, keyLen int) (cert TLSCertPEM, key TLSKeyPEM, err error) {
	if keyLen < 2048 {
		log.Warn("Small key-length:", keyLen)
	}
	priv, _ := rsa.GenerateKey(rand.Reader, keyLen)
	pub := &priv.PublicKey
	x509cert, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		return
	}
	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE",
		Bytes: x509cert})
	key = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return
}

func secureConfig(c *tls.Config) *tls.Config {
	c.CipherSuites = []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}
	c.MinVersion = tls.VersionTLS12
	return c
}

// ConfigClient returns a tls.Config usable for a call to tls.Dial.
func (cert TLSCertPEM) ConfigClient() (*tls.Config, error) {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(cert)
	if !ok {
		return nil, errors.New("Couldn't get root cert.")
	}
	return secureConfig(&tls.Config{
		RootCAs:            roots,
		InsecureSkipVerify: false,
	}), nil
}

// ConfigServer returns a tls.Config usable for a call to tls.Listen.
func (key TLSKeyPEM) ConfigServer(cert TLSCertPEM) (*tls.Config, error) {
	x509cert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		panic(err)
	}
	return secureConfig(&tls.Config{
		Certificates: []tls.Certificate{x509cert},
	}), nil
}

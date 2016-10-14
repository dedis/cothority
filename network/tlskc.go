package network

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"

	"net"

	"crypto/ecdsa"

	"crypto/elliptic"

	"github.com/dedis/cothority/log"
)

// TLSCertPEM is a PEM-encoded certificate for a TLS connection.
type TLSCertPEM string

// TLSKeyPEM is a PEM-encoded private key for a TLS connection.
type TLSKeyPEM string

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
			CommonName:         "*",
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(validYear, 0, 0),
		SubjectKeyId: subjectKeyID,
		// Indicates to use IsCA
		BasicConstraintsValid: true,
		// Is Certificate Authority - can sign keys
		IsCA: true,
		// The IP-addresses this certificate is valid for
		IPAddresses: ips,
		// Extended key usage - the certificate can be used to authenticate the
		// server.
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// Standard key usage - only use for signing the certificate.
		KeyUsage: x509.KeyUsageCertSign,
	}
}

// NewCertKey returns a PEM-encoded certificate and key. If an error occurs,
// both the cert and key are empty.
func NewCertKey(ca *x509.Certificate, keyLen int) (cert TLSCertPEM, key TLSKeyPEM, err error) {
	if keyLen < 256 {
		log.Warn("Small key-length:", keyLen)
	}
	//priv, err := rsa.GenerateKey(rand.Reader, keyLen)
	var priv *ecdsa.PrivateKey
	switch keyLen {
	case 224:
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case 256:
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case 384:
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case 521:
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		err = errors.New("Unknown key-length - chose 224, 256, 384 or 521.")
	}
	if err != nil {
		return
	}
	pub := &priv.PublicKey
	x509cert, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		return
	}
	cert = TLSCertPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE",
		Bytes: x509cert}))
	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return
	}
	key = TLSKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}))
	return
}

func secureConfig(c *tls.Config) *tls.Config {
	// This makes sure that we only get a connection with ECDHE
	// key-exchange.
	c.CipherSuites = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}
	c.MinVersion = tls.VersionTLS12
	return c
}

// ConfigClient returns a tls.Config usable for a call to tls.Dial.
func (cert TLSCertPEM) ConfigClient() (*tls.Config, error) {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(cert))
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
	x509cert, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		panic(err)
	}
	return secureConfig(&tls.Config{
		Certificates: []tls.Certificate{x509cert},
	}), nil
}

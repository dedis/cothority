package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/dedis/onet/log"
	"github.com/ericchiang/letsencrypt"
)

// LetsencryptCert corresponds to the root certificate of let's encrypt
const LetsencryptCert = `-----BEGIN CERTIFICATE-----
MIIEkjCCA3qgAwIBAgIQCgFBQgAAAVOFc2oLheynCDANBgkqhkiG9w0BAQsFADA/
MSQwIgYDVQQKExtEaWdpdGFsIFNpZ25hdHVyZSBUcnVzdCBDby4xFzAVBgNVBAMT
DkRTVCBSb290IENBIFgzMB4XDTE2MDMxNzE2NDA0NloXDTIxMDMxNzE2NDA0Nlow
SjELMAkGA1UEBhMCVVMxFjAUBgNVBAoTDUxldCdzIEVuY3J5cHQxIzAhBgNVBAMT
GkxldCdzIEVuY3J5cHQgQXV0aG9yaXR5IFgzMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAnNMM8FrlLke3cl03g7NoYzDq1zUmGSXhvb418XCSL7e4S0EF
q6meNQhY7LEqxGiHC6PjdeTm86dicbp5gWAf15Gan/PQeGdxyGkOlZHP/uaZ6WA8
SMx+yk13EiSdRxta67nsHjcAHJyse6cF6s5K671B5TaYucv9bTyWaN8jKkKQDIZ0
Z8h/pZq4UmEUEz9l6YKHy9v6Dlb2honzhT+Xhq+w3Brvaw2VFn3EK6BlspkENnWA
a6xK8xuQSXgvopZPKiAlKQTGdMDQMc2PMTiVFrqoM7hD8bEfwzB/onkxEz0tNvjj
/PIzark5McWvxI0NHWQWM6r6hCm21AvA2H3DkwIDAQABo4IBfTCCAXkwEgYDVR0T
AQH/BAgwBgEB/wIBADAOBgNVHQ8BAf8EBAMCAYYwfwYIKwYBBQUHAQEEczBxMDIG
CCsGAQUFBzABhiZodHRwOi8vaXNyZy50cnVzdGlkLm9jc3AuaWRlbnRydXN0LmNv
bTA7BggrBgEFBQcwAoYvaHR0cDovL2FwcHMuaWRlbnRydXN0LmNvbS9yb290cy9k
c3Ryb290Y2F4My5wN2MwHwYDVR0jBBgwFoAUxKexpHsscfrb4UuQdf/EFWCFiRAw
VAYDVR0gBE0wSzAIBgZngQwBAgEwPwYLKwYBBAGC3xMBAQEwMDAuBggrBgEFBQcC
ARYiaHR0cDovL2Nwcy5yb290LXgxLmxldHNlbmNyeXB0Lm9yZzA8BgNVHR8ENTAz
MDGgL6AthitodHRwOi8vY3JsLmlkZW50cnVzdC5jb20vRFNUUk9PVENBWDNDUkwu
Y3JsMB0GA1UdDgQWBBSoSmpjBH3duubRObemRWXv86jsoTANBgkqhkiG9w0BAQsF
AAOCAQEA3TPXEfNjWDjdGBX7CVW+dla5cEilaUcne8IkCJLxWh9KEik3JHRRHGJo
uM2VcGfl96S8TihRzZvoroed6ti6WqEBmtzw3Wodatg+VyOeph4EYpr/1wXKtx8/
wApIvJSwtmVi4MFU5aMqrSDE6ea73Mj2tcMyo5jMd6jmeWUHK8so/joWUoHOUgwu
X4Po1QYz+3dszkDqMp4fklxBwXRsW10KXzPMTZ+sOPAveyxindmjkW8lGy+QsRlG
PfZ+G6Z6h7mjem0Y+iWlkYcV4PIWL1iwBi8saCbGS5jN2p8M+X+Q7UNKEkROb3N6
KOqkqm57TH2H3eDJAkSnh6/DNFu0Qg==
-----END CERTIFICATE-----`

// LetsEncryptURL is the URL needed to create a new client
const LetsEncryptURL = "https://acme-v01.api.letsencrypt.org/directory"

//const LetsEncryptURL = "https://acme-staging-v02.api.letsencrypt.org/directory"

// Convert a certificate of type string into certificate of type x509.Certificate
func pemToCertificate(certPem []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPem)
	if block == nil {
		return nil, errors.New("Fail to decode pem")
	}

	return x509.ParseCertificate(block.Bytes)
}

// Verify if the string given as arguments correspond to a certificate
// Return true if it is the case and false otherwise
func isCert(cert []byte) bool {
	_, err := pemToCertificate(cert)
	return err == nil
}

// Check if the string in argument correspond to a valid certificate
// Return true if it is the case and false otherwise
func isValid(cert []byte) bool {
	rootPool := x509.NewCertPool()
	certRoots, err := pemToCertificate([]byte(LetsencryptCert))
	if err != nil {
		return false
	}
	rootPool.AddCert(certRoots)
	certCheck, err := pemToCertificate(cert)
	if err != nil {
		return false
	}
	opts := x509.VerifyOptions{
		DNSName: certCheck.DNSNames[0],
		Roots:   rootPool,
	}
	if _, err := certCheck.Verify(opts); err != nil {
		return false
	}
	log.Info("Certificate OK")
	return true
}

// Renew the certificate that is characterized by its string version
// and returns the new certificate
func renewCert(cert []byte) ([]byte, error) {
	oldcert, err := pemToCertificate(cert)
	if err != nil {
		return nil, err
	}

	// compute the URI with seial
	certuri := fmt.Sprintf("https://acme-v01.api.letsencrypt.org/acme/cert/0%x", oldcert.SerialNumber)

	// Create a client to the acme of letsencrypt.
	cli, err := letsencrypt.NewClient(LetsEncryptURL)
	if err != nil {
		return nil, err
	}

	// Renew the certificate
	certRen, err := cli.RenewCertificate(certuri)
	if err != nil {
		return nil, err
	}

	// Write in the memory
	fullchain, err := cli.Bundle(certRen)
	if err != nil {
		return nil, errors.New("can't bundle the certificate" + err.Error())
	}

	return fullchain, nil
}

// Revoke a certificate by giving as arguments the certificate and the path to
// the registerkey.pem key. This key is created when registering to the ACME
// server
func revokeCert(path string, cert []byte) error {
	// Create a client to the acme of letsencrypt
	log.Info("Revoking the key")
	cli, err := letsencrypt.NewClient(LetsEncryptURL)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err != nil {
		return err
	}

	// Find or generate the private key and register with the ACME server.
	var key *rsa.PrivateKey
	key, err = loadKey(path)
	if err != nil {
		return err
	}

	// Revoke the certificate
	err = cli.RevokeCertificate(key, cert)
	if err != nil {
		return err
	}

	return nil
}

// Code adapted from: https://ericchiang.github.io/post/go-letsencrypt/
// Request a certificate by first registering to the ACME server and then by
// completing a challenge then it returns the certificate as string type
func getCert(wwwDir string, certDir string, domain string) ([]byte, error) {
	certPath := path.Join(certDir, domain)

	// Create a client to the acme of letsencrypt.
	log.Info("Requesting RSA keys")
	cli, err := letsencrypt.NewClient(LetsEncryptURL)
	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(certPath); os.IsNotExist(err) {
		err = os.MkdirAll(certPath, 0777)
	}
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(wwwDir); os.IsNotExist(err) {
		return nil, err
	}

	// Find or generate the private key to register on the ACME server.
	var key *rsa.PrivateKey
	key, err = loadKey(path.Join(certPath, "registerkey.pem"))
	if err != nil {
		log.Info("New RSA key generation for registering, keep it safe")
		key, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		ioutil.WriteFile(path.Join(certPath, "registerkey.pem"), pem.EncodeToMemory(&pem.Block{Bytes: x509.MarshalPKCS1PrivateKey(key), Type: "RSA PRIVATE KEY"}), 0644)
	}

	// Register to the ACME server
	if _, err := cli.NewRegistration(key); err != nil {
		return nil, err
	}

	log.Info("Registration successful!")

	// Get the http-challenge
	auth, _, err := cli.NewAuthorization(key, "dns", domain)
	if err != nil {
		return nil, err
	}
	httpChalURL := ""
	for _, chal := range auth.Challenges {
		if chal.Type == "http-01" {
			httpChalURL = chal.URI
		}
	}
	if httpChalURL == "" {
		return nil, errors.New("http-01 challenge not found")
	}
	chal, err := cli.Challenge(httpChalURL)
	if err != nil {
		return nil, err
	}

	// Get the path and the resource needed to perform the challenge
	pathChallenge, resource, err := chal.HTTP(key)
	if err != nil {
		return nil, err
	}

	// Copy the resource for the challenge
	if os.MkdirAll(path.Join(wwwDir, ".well-known/acme-challenge/"), 0711) != nil {
		return nil, errors.New("Problem creating new dir for challenge : " + err.Error())
	}
	if err := ioutil.WriteFile(path.Join(wwwDir, pathChallenge), []byte(resource), 0644); err != nil {
		return nil, err
	}

	// Complete the challenge
	if err := cli.ChallengeReady(key, chal); err != nil {
		return nil, errors.New("Challenge not complete " + err.Error())
	}
	log.Info("Challenge Complete")

	// Create a certificate request
	csr, _, err := newCSR(domain, certPath)
	if err != nil {
		return nil, err
	}

	// Request a certificate for the domain
	cert, err := cli.NewCertificate(key, csr)
	if err != nil {
		return nil, err
	}
	log.Info("New Certificate Successfuly created")

	fullchain, err := cli.Bundle(cert)
	if err != nil {
		return nil, errors.New("Can't bundle the certificate" + err.Error())
	}

	err = ioutil.WriteFile(path.Join(certPath, "fullchain.pem"), fullchain, 0644)
	if err != nil {
		return nil, err
	}

	return fullchain, nil
}

// Code adapted from: https://ericchiang.github.io/post/go-letsencrypt/
// Generate a new certificate signing request by giving the domain of our server
// and create the private key in the process. This request will be needed to
// finalise the creation of our certificate
func newCSR(domain string, certPath string) (*x509.CertificateRequest, *rsa.PrivateKey, error) {
	var certKey *rsa.PrivateKey
	certKey, err := loadKey(path.Join(certPath, "privkey.pem"))
	if err != nil {
		log.Info("New RSA private key generation for the certificate, keep it safe")
		certKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, nil, err
		}
		err = ioutil.WriteFile(path.Join(certPath, "privkey.pem"), pem.EncodeToMemory(&pem.Block{Bytes: x509.MarshalPKCS1PrivateKey(certKey), Type: "RSA PRIVATE KEY"}), 0644)
		if err != nil {
			return nil, nil, err
		}
	}
	template := &x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		PublicKey:          &certKey.PublicKey,
		Subject:            pkix.Name{CommonName: domain},
		DNSNames:           []string{domain},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, certKey)
	if err != nil {
		return nil, nil, err
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, nil, err
	}
	return csr, certKey, nil
}

// Load the key corresponding to the file name given as argumento into a rsa.PrivateKey
// Returns the rsa.PrivateKey corresponding to the file name given as argument
func loadKey(file string) (*rsa.PrivateKey, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("pem decode: no key found")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// Utility method that splits the fullchain certificate into the domain certificate
// and the chain certificate
func splitCertPublicChain(cert string) (string, string) {
	s := []string{"", ""}

	if cert != "" {
		s = strings.SplitAfter(cert, "-----END CERTIFICATE-----")
	}
	return s[0], strings.TrimPrefix(s[1], "\n")
}

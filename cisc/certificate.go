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
	"strings"

	"github.com/dedis/onet/log"
	"github.com/ericchiang/letsencrypt"
)

// LetsencryptCert let's encrypt certificate
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

// Compare and prints the difference between two certificates
// It prints the difference between the 'NotAfter', 'Subject.CommonName',
// 'Issuer.CommonName' and 'PublicKey'
// Not used
/*func compare(cOld, cNew, k string) {
	log.Info("Compare old against new cert: " + k)
	cert1, err := pemToCertificate(cOld)
	if err != nil {
		log.Info("the old value is not a cert")
		return
	}
	cert2, err := pemToCertificate(cNew)
	if err != nil {
		log.Info("the new value is not a cert")
		return
	}
	if cert1.Equal(cert2) {
		log.Info("No change in the certificate")

	} else {
		log.Info("The new certificate is different :")
		if !((cert1.NotAfter).Equal(cert2.NotAfter)) {
			log.Info("    Expiration date modification: " + cert1.NotAfter.String() + " to " + cert2.NotAfter.String())
		}
		if !((cert1.Subject.CommonName) == (cert2.Subject.CommonName)) {
			log.Info("    Subject modification: " + cert1.Subject.CommonName + " to " + cert2.Subject.CommonName)
		}
		if !((cert1.Issuer.CommonName) == (cert2.Issuer.CommonName)) {
			log.Info("    Issuer modification: " + cert1.Issuer.CommonName + " to " + cert2.Issuer.CommonName)
		}

		if !((cert1.PublicKey.(*rsa.PublicKey).N).Cmp(cert2.PublicKey.(*rsa.PublicKey).N) == 0) {
			log.Info("    PublicKey modification")
		}
	}
}*/

// Convert a certificate of type string into certificate of type x509.Certificate
func pemToCertificate(certPem string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPem))
	if block == nil {
		return nil, errors.New("Fail to decode pem")
	}

	return x509.ParseCertificate(block.Bytes)
}

// Verify if the string given as arguments correspond to a certificate
// Return true if it is the case and return false otherwise
func isCert(cert string) bool {
	_, err := pemToCertificate(cert)
	return err == nil
}

// Check if the string in argument correspond to a valid certificate
// Return true if it is the case and return false otherwise
func isValid(cert string) bool {
	rootPool := x509.NewCertPool()
	certRoots, err := pemToCertificate(LetsencryptCert)
	if err != nil {
		log.Info("The root certificate is not valid")
		return false
	}
	rootPool.AddCert(certRoots)
	certCheck, err := pemToCertificate(cert)
	if err != nil {
		log.Info("The checked certificate is not valid")
		return false
	}
	opts := x509.VerifyOptions{
		DNSName: certCheck.DNSNames[0],
		Roots:   rootPool,
	}
	if _, err := certCheck.Verify(opts); err != nil {
		log.Info("The certificate is not valid: " + err.Error())
		return false
	}
	log.Info("Certificate OK")
	return true
}

// Renew the a certificate that is caracterized by its string version
// Returns the renewed fullchain.pem
func renewCert(cert string) (string, error) {

	oldcert, err := pemToCertificate(cert)
	if err != nil {
		return "", err
	}

	// Get the serial number and compute the URI
	serial := fmt.Sprintf("%x", oldcert.SerialNumber)
	certuri := "https://acme-v01.api.letsencrypt.org/acme/cert/0" + serial

	// Create a client to the acme of letsencrypt.
	//cli, err := letsencrypt.NewClient("https://acme-v01.api.letsencrypt.org/directory")
	cli, err := letsencrypt.NewClient("https://acme-staging.api.letsencrypt.org/directory")
	if err != nil {
		return "", err
	}

	// Renew the certificate
	certRen, err := cli.RenewCertificate(string(certuri))
	if err != nil {
		return "", err
	}

	// Write in the memory
	fullchain, err := cli.Bundle(certRen)
	if err != nil {
		return "", errors.New("can't bundle the certificate" + err.Error())
	}
	ioutil.WriteFile("fullchain.pem", fullchain, 0644)
	log.Info("Certificate successfully renewed")

	return string(fullchain), nil
}

// Revoke a certificate by giving as arguments the path to the registerkey.pem
// this key is created when registering to the ACME server and the string corresponding
// to the certificate
// Returns nothing
func revokeCert(path string, cert string) error {

	// Create a client to the acme of letsencrypt
	log.Info("Revoking the key")
	//cli, err := letsencrypt.NewClient("https://acme-v01.api.letsencrypt.org/directory")
	cli, err := letsencrypt.NewClient("https://acme-staging.api.letsencrypt.org/directory")
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return errors.New("key can not be found")
		}
	}

	// Find or generate the private key and register with the ACME server.
	var key *rsa.PrivateKey
	key, err = loadKey(path)
	if err != nil {
		return err
	}

	// Revoke the certificate
	err = cli.RevokeCertificate(key, []byte(cert))
	if err != nil {
		return err
	}
	log.Info("Succesfully revoked")
	return nil
}

// Code adapted from: https://ericchiang.github.io/post/go-letsencrypt/
// Request a certificate by first registering to the ACME server and then by completing
// a challenge
// Returns the certificate as string type
func getCert(dir string, domain string) (string, error) {

	// Create a client to the acme of letsencrypt.
	log.Info("Requesting RSA keys")
	//cli, err := letsencrypt.NewClient("https://acme-v01.api.letsencrypt.org/directory")
	cli, err := letsencrypt.NewClient("https://acme-staging.api.letsencrypt.org/directory")
	if err != nil {
		return "", err
	}

	if _, err = os.Stat("/home/" + dir + "/cert/" + domain); os.IsNotExist(err) {
		os.MkdirAll("/home/"+dir+"/cert/"+domain, 0777)
	}

	// Find or generate the private key to register on the ACME server.
	var key *rsa.PrivateKey
	key, err = loadKey("/home/" + dir + "/cert/" + domain + "/registerkey.pem")
	if err != nil {
		log.Info("New RSA key generation for registering, keep it safe")
		key, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return "", err
		}
		ioutil.WriteFile("/home/"+dir+"/cert/"+domain+"/registerkey.pem", pem.EncodeToMemory(&pem.Block{Bytes: x509.MarshalPKCS1PrivateKey(key), Type: "RSA PRIVATE KEY"}), 0644)
	}

	// Register to the ACME server
	if _, err := cli.NewRegistration(key); err != nil {
		return "", err
	}

	log.Info("Registration successful!")

	// Get the http-challenge
	auth, _, err := cli.NewAuthorization(key, "dns", domain)
	if err != nil {
		return "", err
	}
	httpChalURL := ""
	for _, chal := range auth.Challenges {
		if chal.Type == "http-01" {
			httpChalURL = chal.URI
		}
	}
	if httpChalURL == "" {
		return "", errors.New("http-01 challenge not found")
	}
	chal, err := cli.Challenge(httpChalURL)
	if err != nil {
		return "", err
	}

	// Get the path and the resource needed to perform the challenge
	path, resource, err := chal.HTTP(key)
	if err != nil {
		return "", err
	}

	// Copy the resource for the challenge
	if os.MkdirAll("/home/"+dir+"/www/.well-known/acme-challenge/", 0711) != nil {
		return "", errors.New("Problem creating new dir for challenge : " + err.Error())
	}
	if err := ioutil.WriteFile("/home/"+dir+"/www/"+path, []byte(resource), 0644); err != nil {
		return "", err
	}

	// Complete the challenge
	if err := cli.ChallengeReady(key, chal); err != nil {
		return "", errors.New("Challenge not complete" + err.Error())
	}
	log.Info("Challenge Complete")

	// Create a certificate request
	csr, _, err := newCSR(domain, dir)
	if err != nil {
		return "", err
	}

	// Request a certificate for the domain
	cert, err := cli.NewCertificate(key, csr)

	if err != nil {
		return "", err
	}
	log.Info("New Certificate Successfuly created")
	fullchain, err := cli.Bundle(cert)

	if err != nil {
		return "", errors.New("Can't bundle the certificate" + err.Error())
	}
	ioutil.WriteFile("/home/"+dir+"/cert/"+domain+"/fullchain.pem", fullchain, 0644)

	return string(fullchain), nil
}

// Code adapted from: https://ericchiang.github.io/post/go-letsencrypt/
// TODO
func newCSR(domain string, dir string) (*x509.CertificateRequest, *rsa.PrivateKey, error) {
	var certKey *rsa.PrivateKey
	certKey, err := loadKey("/home/" + dir + "/cert/" + domain + "/privkey.pem")
	if err != nil {
		log.Info("New RSA private key generation for the certificate, keep it safe")
		certKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, nil, err
		}
		ioutil.WriteFile("/home/"+dir+"/cert/"+domain+"/privkey.pem", pem.EncodeToMemory(&pem.Block{Bytes: x509.MarshalPKCS1PrivateKey(certKey), Type: "RSA PRIVATE KEY"}), 0644)
	}
	if err != nil {
		return nil, nil, err
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

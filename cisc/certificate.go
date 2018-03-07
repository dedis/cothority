package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/dedis/onet/log"
	"github.com/ericchiang/letsencrypt"
	"io/ioutil"
	"os"
)

const Letsencrypt_cert = `-----BEGIN CERTIFICATE-----
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

func compare(c_old, c_new, k string) {
	log.Info("Compare old against new cert: " + k)
	cert_1, err := pemToCertificate(c_old)
	if err != nil {
		log.Info("the old value is not a cert")
		return
	}
	cert_2, err := pemToCertificate(c_new)
	if err != nil {
		log.Info("the new value is not a cert")
		return
	}
	if cert_1.Equal(cert_2) {
		log.Info("No change in the certificate")

	} else {
		log.Info("The new certificate is different :")
		if !((cert_1.NotAfter).Equal(cert_2.NotAfter)) {
			log.Info("    Expiration date modification: " + cert_1.NotAfter.String() + " to " + cert_2.NotAfter.String())
		}
		if !((cert_1.Subject.CommonName) == (cert_2.Subject.CommonName)) {
			log.Info("    Subject modification: " + cert_1.Subject.CommonName + " to " + cert_2.Subject.CommonName)
		}
		if !((cert_1.Issuer.CommonName) == (cert_2.Issuer.CommonName)) {
			log.Info("    Issuer modification: " + cert_1.Issuer.CommonName + " to " + cert_2.Issuer.CommonName)
		}

		if !((cert_1.PublicKey.(*rsa.PublicKey).N).Cmp(cert_2.PublicKey.(*rsa.PublicKey).N) == 0) {
			log.Info("    PublicKey modification")
		}
	}
}

func pemToCertificate(cert_pem string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(cert_pem))
	if block == nil {
		return nil, errors.New("Fail decode pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func isCert(cert string) bool {
	_, err := pemToCertificate(cert)
	return err == nil
}
func check(cert string) bool {
	root_pool := x509.NewCertPool()
	cert_roots, err := pemToCertificate(Letsencrypt_cert)
	if err != nil {
		log.Info("The root cert is not valid")
	}
	root_pool.AddCert(cert_roots)
	cert_check, err := pemToCertificate(cert)
	if err != nil {
		log.Info("The checked cert is not valid")
	}
	opts := x509.VerifyOptions{
		DNSName: cert_check.DNSNames[0],
		Roots:   root_pool,
	}
	if _, err := cert_check.Verify(opts); err != nil {
		log.Info("certificate not valid: " + err.Error())
		return false
	} else {
		log.Info("Cert OK")
		return true
	}

}
func renewCert(cert string) string {

	oldcert, err := pemToCertificate(cert)
	if err != nil {
		log.Fatal(err)
	}
	//get the serial number and compute the URI
	serial := fmt.Sprintf("%x", oldcert.SerialNumber)
	certuri := "https://acme-v01.api.letsencrypt.org/acme/cert/0" + serial
	// Create a client to the acme of letsencrypt.
	cli, err := letsencrypt.NewClient("https://acme-v01.api.letsencrypt.org/directory")
	if err != nil {
		log.Fatal(err)
	}
	//renew the certificate
	cert_ren, err := cli.RenewCertificate(string(certuri))
	if err != nil {
		log.Fatal("Problem while renewing Certificate:" + err.Error())
	}
	//write in the memory
	fullchain, err := cli.Bundle(cert_ren)
	if err != nil {
		log.Fatal("can't bundles the certificate", err)
	}
	ioutil.WriteFile("fullchain.pem", fullchain, 0644)
	log.Info("Certificate successfully renewed")
	//return the cert
	cert_PEM := pem.EncodeToMemory(&pem.Block{Bytes: cert_ren.Certificate.Raw, Type: "CERTIFICATE"})
	return string(cert_PEM)
}

func revokeCert(cert string) {
	// Create a client to the acme of letsencrypt.
	cli, err := letsencrypt.NewClient("https://acme-v01.api.letsencrypt.org/directory")
	if err != nil {
		log.Fatal(err)
	}
	// Find or generate the private key and register with the ACME server.
	var key *rsa.PrivateKey
	key, err = loadKey("registerkey.pem")
	if err != nil {
		log.Print("new generate RSA key for register , keep it safe")
		key, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatal(err)
		}
		ioutil.WriteFile("registerkey.pem", pem.EncodeToMemory(&pem.Block{Bytes: x509.MarshalPKCS1PrivateKey(key), Type: "RSA PRIVATE KEY"}), 0644)
	}

	//register to the ACME server
	if _, err := cli.NewRegistration(key); err != nil {
		log.Fatal("Can not Register to the ACME:", err)
	}
	//revoke the certificate
	re_err := cli.RevokeCertificate(key, []byte(cert))
	if re_err != nil {
		log.Fatal("error while revoke:", re_err)
	}
	log.Print("Succesfully revoked")
}

//code addapted from :https://ericchiang.github.io/post/go-letsencrypt/ "
func getCert(domain string) string {
	// Create a client to the acme of letsencrypt.
	cli, err := letsencrypt.NewClient("https://acme-v01.api.letsencrypt.org/directory")
	if err != nil {
		log.Fatal(err)
	}
	// Find or generate the private key to register on the ACME server.
	var key *rsa.PrivateKey
	key, err = loadKey("registerkey.pem")
	if err != nil {
		log.Print("new generate RSA key for register , keep it safe")
		key, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatal(err)
		}
		ioutil.WriteFile("registerkey.pem", pem.EncodeToMemory(&pem.Block{Bytes: x509.MarshalPKCS1PrivateKey(key), Type: "RSA PRIVATE KEY"}), 0644)
	}

	//register to the ACME server
	if _, err := cli.NewRegistration(key); err != nil {
		log.Fatal(err)
	}

	log.Print("Registration successful!")

	//Get the http-challenge
	auth, _, err := cli.NewAuthorization(key, "dns", domain)
	if err != nil {
		log.Fatal(err)
	}
	httpChalURL := ""
	for _, chal := range auth.Challenges {
		if chal.Type == "http-01" {
			httpChalURL = chal.URI
		}
	}
	if httpChalURL == "" {
		log.Fatal("Not found http-01 challenge")
	}
	chal, err := cli.Challenge(httpChalURL)
	if err != nil {
		log.Fatal(err)
	}

	//Get the path and the resource needed to perform the challenge
	path, resource, err := chal.HTTP(key)
	if err != nil {
		log.Fatal(err)
	}
	//copy the resource for the challenge
	if os.MkdirAll("./.well-known/acme-challenge/", 0711) != nil {
		log.Fatal("problem creating new dir for challenge :", err)
	}
	if err := ioutil.WriteFile("."+path, []byte(resource), 0644); err != nil {
		log.Fatal(err)
	}
	//complete the challenge
	if err := cli.ChallengeReady(key, chal); err != nil {
		log.Fatal("Challenge not complete :", err)
	}
	log.Print("Challenge Complete")
	//create a certificate request
	csr, _, err := newCSR(domain)
	if err != nil {
		log.Fatal(err)
	}
	// Request a certificate for the domain
	cert, err := cli.NewCertificate(key, csr)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("New Certificate Successfuly created")
	cert_PEM := pem.EncodeToMemory(&pem.Block{Bytes: cert.Certificate.Raw, Type: "CERTIFICATE"})
	fullchain, err := cli.Bundle(cert)
	if err != nil {
		log.Fatal("can't bundles the certificate:", err)
	}
	ioutil.WriteFile("fullchain.pem", fullchain, 0644)
	return string(cert_PEM)
}

//code adapted from :https://ericchiang.github.io/post/go-letsencrypt/ "
func newCSR(domain string) (*x509.CertificateRequest, *rsa.PrivateKey, error) {
	var certKey *rsa.PrivateKey
	certKey, err := loadKey("privkey.pem")
	if err != nil {
		log.Print("new RSA private key generate for the certificate, keep it safe")
		certKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			log.Fatal(err)
		}
		ioutil.WriteFile("privkey.pem", pem.EncodeToMemory(&pem.Block{Bytes: x509.MarshalPKCS1PrivateKey(certKey), Type: "RSA PRIVATE KEY"}), 0644)
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

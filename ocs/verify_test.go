package ocs

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"testing"
)

const (
	rootCert1 = `-----BEGIN CERTIFICATE-----
MIIB1jCCATigAwIBAgIBATAKBggqhkjOPQQDBDAdMRswGQYDVQQDExJCeXpHZW4g
c2lnbmVyIG9yZzEwHhcNMTkwMzI4MjEwNzUxWhcNNDQwMzIxMjEwNzUxWjAdMRsw
GQYDVQQDExJCeXpHZW4gc2lnbmVyIG9yZzEwgZswEAYHKoZIzj0CAQYFK4EEACMD
gYYABABqdo+aDVte5Fz/xG5Z2GYmIbcVJdXxrMJrTBYgHQafSw0BBKrAyeMcZ534
/V6eNfkiZa3kuflo6Y2E/NtVxyl7dgFBYTdqvLtPdg7+K7pdj8eKFrAQ0DDi5S0x
aM96oR3S0bU4MIbfMqW1fAsLPw3476Gvju73bfJhEJ3ukx6W2olq+KMmMCQwDgYD
VR0PAQH/BAQDAgIEMBIGA1UdEwEB/wQIMAYBAf8CAQEwCgYIKoZIzj0EAwQDgYsA
MIGHAkEcvPgm0qnXMgpJiOD52VUL3qTwU6uzRYhwIWa3sWCP471/muzsq6PctAEu
CHkpnAlH3DuS2MBBql8ifwwK2PdOGQJCAQcE3+qdiyrABJ315INCTu6HAjpGv0cR
VQWcCmSs80tS9gzvQJ8+peWRuzGvy1Uoyj0qHTSJOHx6z86oOIVbXAIj
-----END CERTIFICATE-----`

	validPem = `-----BEGIN CERTIFICATE-----
MIICKTCCAYqgAwIBAgIQYNsgS2KrQ1ptA7E+cRfiUjAKBggqhkjOPQQDBDAdMRsw
GQYDVQQDExJCeXpHZW4gc2lnbmVyIG9yZzEwHhcNMTkwMzI4MjEwNzUxWhcNMTkw
NDExMjEwNzUxWjAoMSYwJAYDVQQDDB1FcGhlbWVyYWwgcmVhZCBvcGVyYXRpb24g
JiBDbzB2MBAGByqGSM49AgEGBSuBBAAiA2IABPEbevkxsAu3BqZjMBzl+ppSLX1F
4oqnAUxmXx+Yw9mgyunTWzHKPAgHoYmaVDL2a+MDVngmbJI+BiXaZBE00gW854pz
ROa1Z7KxjYGgbRINavXX5nSTbs+xH3w76d3ppKOBgzCBgDAOBgNVHQ8BAf8EBAMC
BSAwDAYDVR0TAQH/BAIwADAvBggrBgEFBQcNAQEB/wQg7PBd8YGomyUmjZpqOy9h
gdAdKEfArphKLRkkozsRRvIwLwYIKwYBBQUHDQIBAf8EIBuJzdwW5DfOVymjPvBM
YXsz+apB9URZnhN1jZy2wrixMAoGCCqGSM49BAMEA4GMADCBiAJCAYwxRrOwCydO
r5KoAndH8/U9nIaM4BWcx1pwYFMM44P0BzXDQgDSYwIAhAQ5hvOpaMPB4IMKI37C
G1lsOKivZEboAkIA90UbyVD7ahZdbpCDKUYAoVejKgA5JAsm8kUGPWt+siw2hsT9
V/NTETY3evBjoX8kkWs/E5pWpwEGKPQaS25gw1s=
-----END CERTIFICATE-----`
)

func Test_VerifyCertificateHappyDayScenario(t *testing.T) {
	caCert, _ := certFromPem([]byte(rootCert1))
	cert, _ := certFromPem([]byte(validPem))

	writeId, key, _ := Verify(caCert, cert)
	t.Log("writeId", hex.EncodeToString(writeId))
	t.Log("key", hex.EncodeToString(key))
}

func certFromPem(pemCerts []byte) (cert *x509.Certificate, err error) {
	var block *pem.Block

	block, pemCerts = pem.Decode(pemCerts)

	if block.Type != "CERTIFICATE" {
		return nil, errors.New("expected a certificate")
	}

	return x509.ParseCertificate(block.Bytes)
}

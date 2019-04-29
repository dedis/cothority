package ocs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"strings"
	"time"

	"go.dedis.ch/onet/v3"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3/log"
)

// EncodeKey can be used by the writer to an onchain-secret skipchain
// to encode his symmetric key under the collective public key created
// by the DKG.
// As this method uses `Pick` to encode the key, depending on the key-length
// more than one point is needed to encode the data.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - key - the symmetric key for the document
//
// Output:
//   - U - the schnorr commit
//   - C - encrypted key
func EncodeKey(suite suites.Suite, X kyber.Point, key []byte) (U kyber.Point, C kyber.Point, err error) {
	if len(key) > suite.Point().EmbedLen() {
		return nil, nil, errors.New("got more data than can fit into one point")
	}
	r := suite.Scalar().Pick(suite.RandomStream())
	C = suite.Point().Mul(r, X)
	log.Lvl3("C:", C.String())
	U = suite.Point().Mul(r, nil)
	log.Lvl3("U is:", U.String())

	kp := suite.Point().Embed(key, suite.RandomStream())
	log.Lvl3("Keypoint:", kp.String())
	log.Lvl3("X:", X.String())
	C.Add(C, kp)
	return
}

// DecodeKey can be used by the reader of an onchain-secret to convert the
// re-encrypted secret back to a symmetric key that can be used later to
// decode the document.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - C - the encrypted key
//   - XhatEnc - the re-encrypted schnorr-commit
//   - xc - the private key of the reader
//
// Output:
//   - key - the re-assembled key
//   - err - an eventual error when trying to recover the data from the points
func DecodeKey(suite kyber.Group, X kyber.Point, C kyber.Point, XhatEnc kyber.Point,
	xc kyber.Scalar) (key []byte, err error) {
	log.Lvl3("xc:", xc)
	xcInv := suite.Scalar().Neg(xc)
	log.Lvl3("xcInv:", xcInv)
	sum := suite.Scalar().Add(xc, xcInv)
	log.Lvl3("xc + xcInv:", sum, "::", xc)
	log.Lvl3("X:", X)
	XhatDec := suite.Point().Mul(xcInv, X)
	log.Lvl3("XhatDec:", XhatDec)
	log.Lvl3("XhatEnc:", XhatEnc)
	Xhat := suite.Point().Add(XhatEnc, XhatDec)
	log.Lvl3("Xhat:", Xhat)
	XhatInv := suite.Point().Neg(Xhat)
	log.Lvl3("XhatInv:", XhatInv)

	// Decrypt C to keyPointHat
	log.Lvl3("C:", C)
	keyPointHat := suite.Point().Add(C, XhatInv)
	log.Lvl3("keyPointHat:", keyPointHat)
	key, err = keyPointHat.Data()
	if err != nil {
		return nil, Erret(err)
	}
	log.Lvl3("key:", key)
	return
}

func Erret(err error) error {
	if err == nil {
		return nil
	}
	pc, _, line, _ := runtime.Caller(1)
	errStr := err.Error()
	if strings.HasPrefix(errStr, "Erret") {
		errStr = "\n\t" + errStr
	}
	return fmt.Errorf("Erret at %s: %d -> %s", runtime.FuncForPC(pc).Name(), line, errStr)
}

// Helper functions to create x509-certificates.
//
//   CertNode - can be given as a CA for Reencryption and Resharing
//   +-> CertReencrypt - indicates who is allowed to reencrypt and gives the ephemeral key

// BCCert is used as a structure in testing - this is not secure enough to be used in production.
type BCCert struct {
	Private     *ecdsa.PrivateKey
	Certificate *x509.Certificate
}

// NewBCCert is the general method to create a certificate for testing.
func NewBCCert(cn string, dur time.Duration, kus x509.KeyUsage, isCA bool,
	eext []pkix.Extension, root *x509.Certificate, rootPriv *ecdsa.PrivateKey) BCCert {
	notBefore := time.Now()
	notAfter := notBefore.Add(dur)
	serialNumber := big.NewInt(int64(1))

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              kus,
		BasicConstraintsValid: true,
		MaxPathLen:            2,
		IsCA:                  isCA,
	}
	if eext != nil {
		template.ExtraExtensions = eext
	}
	bcc := BCCert{}
	var err error
	bcc.Private, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	log.ErrFatal(err)
	if root == nil {
		root = &template
		rootPriv = bcc.Private
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, root, &bcc.Private.PublicKey, rootPriv)
	log.ErrFatal(err)

	bcc.Certificate, err = x509.ParseCertificate(derBytes)
	log.ErrFatal(err)
	return bcc
}

// CADur is the duration for a CA - here artificially restricted to 24 hours, because it is for testing only.
var CADur = 24 * time.Hour

// NewBCCA creates a CA cert.
func NewBCCA(cn string) BCCert {
	return NewBCCert(cn, CADur, x509.KeyUsageCertSign|x509.KeyUsageDataEncipherment, true,
		nil, nil, nil)
}

// CreateSubCA creates a CA that is signed by the CA of the given bcc.
func (bcc BCCert) CreateSubCA(cn string) BCCert {
	return NewBCCert(cn, CADur, x509.KeyUsageCertSign|x509.KeyUsageDataEncipherment, true,
		nil, bcc.Certificate, bcc.Private)
}

// Sign is a general signing method that creates a new certificate, which is not a CA.
func (bcc BCCert) Sign(cn string, eext []pkix.Extension) BCCert {
	return NewBCCert(cn, time.Hour, x509.KeyUsageKeyEncipherment, false, eext, bcc.Certificate, bcc.Private)
}

// Reencrypt is a specific reencryption certificate created with extrafields that are used by Calypso.
func (bcc BCCert) Reencrypt(writeID []byte, ephemeralPublicKey kyber.Point) BCCert {
	writeIdExt := pkix.Extension{
		Id:       OIDWriteId,
		Critical: true,
		Value:    writeID,
	}

	ephemeralKeyExt := pkix.Extension{
		Id:       OIDEphemeralKey,
		Critical: true,
	}
	var err error
	ephemeralKeyExt.Value, err = ephemeralPublicKey.MarshalBinary()
	log.ErrFatal(err)

	return bcc.Sign("reencryt", []pkix.Extension{writeIdExt, ephemeralKeyExt})
}

// CreateOCS returns a certificate that can be used to authenticate for OCS creation.
func (bcc BCCert) CreateOCS(policyReencrypt, policyReshare *PolicyX509Cert, roster onet.Roster) BCCert {
	pReencBuf, err := protobuf.Encode(policyReencrypt)
	log.ErrFatal(err)
	pReshareBuf, err := protobuf.Encode(policyReshare)
	log.ErrFatal(err)
	rosterBuf, err := protobuf.Encode(&roster)
	log.ErrFatal(err)
	return bcc.Sign("createOCS", []pkix.Extension{
		{
			Id:       OIDPolicyReencrypt,
			Critical: true,
			Value:    pReencBuf,
		},
		{
			Id:       OIDPolicyReshare,
			Critical: true,
			Value:    pReshareBuf,
		},
		{
			Id:       OIDRoster,
			Critical: true,
			Value:    rosterBuf,
		},
	})
}

// CreateCertCa is used for tests and returns a new private key, as well as a CA certificate.
func CreateCertCa() (caPrivKey *ecdsa.PrivateKey, cert *x509.Certificate, err error) {
	bcc := NewBCCA("ByzGen signer org1")
	return bcc.Private, bcc.Certificate, nil
}

// CreateCertReencrypt is used for tests and can create a certificate for a reencryption request.
func CreateCertReencrypt(caCert *x509.Certificate, caPrivKey *ecdsa.PrivateKey,
	writeID []byte, ephemeralPublicKey kyber.Point) (*x509.Certificate, error) {
	bcc := BCCert{Certificate: caCert, Private: caPrivKey}.Reencrypt(writeID, ephemeralPublicKey)
	return bcc.Certificate, nil
}

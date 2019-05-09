// Demo of how the new OCS service works from an outside, non-go-test caller. It does the following steps:
//  1. set up a root CA that is stored in the service as being allowed to create new OCS-instances
//  2. Create a new OCS-instance with a reencryption policy being set by a node-certificate
//  3. Encrypt a symmetric key to the OCS-instance public key
//  4. Ask the OCS-instance to re-encrypt the key to an ephemeral key
//  5. Decrypt the symmetric key
package main

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"strings"

	"go.dedis.ch/cothority/v3/ocs/edwards25519"

	"go.dedis.ch/kyber/v3"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/ocs"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3/log"
)

func main() {
	// Use our own ed25519 suite to be able to print x coordinates:
	cothority.Suite = edwards25519.NewBlakeSHA256Ed25519()
	if len(os.Args) < 2 {
		log.Error("Please give a roster.toml as first parameter")
		printSamples()
		return
	}
	roster, err := lib.ReadRoster(os.Args[1])
	log.ErrFatal(err)

	log.Info("1. Creating createOCS cert and setting OCS-create policy")
	cl := ocs.NewClient()
	caCreate1 := ocs.NewBCCA("Create OCS - 1")
	caCreate2 := ocs.NewBCCA("Create OCS - 2")
	for _, si := range roster.List {
		err = cl.AddPolicyCreateOCS(si, ocs.PolicyCreate{X509Cert: &ocs.PolicyX509Cert{
			CA:        [][]byte{caCreate1.Certificate.Raw, caCreate2.Certificate.Raw},
			Threshold: 2,
		}})
		log.ErrFatal(err)
	}

	log.Info("2.a) Creating new OCS")
	caReenc1 := ocs.NewBCCA("Reencrypt - 1")
	caReenc2 := ocs.NewBCCA("Reencrypt - 2")
	pxReenc := ocs.PolicyReencrypt{
		X509Cert: &ocs.PolicyX509Cert{
			CA: [][]byte{caReenc1.Certificate.Raw,
				caReenc2.Certificate.Raw},
			Threshold: 2,
		},
	}
	pxReshare := ocs.PolicyReshare{
		X509Cert: &ocs.PolicyX509Cert{
			CA: [][]byte{caReenc1.Certificate.Raw,
				caReenc2.Certificate.Raw},
			Threshold: 2,
		},
	}
	cert1 := caCreate1.CreateOCS(pxReenc.X509Cert, pxReshare.X509Cert, *roster).Certificate.Raw
	cert2 := caCreate2.CreateOCS(pxReenc.X509Cert, pxReshare.X509Cert, *roster).Certificate.Raw
	authCreate := ocs.AuthCreate{
		X509Cert: &ocs.AuthCreateX509Cert{
			Certificates: [][]byte{cert1, cert2},
		},
	}

	ocsID, err := cl.CreateOCS(*roster, authCreate, pxReenc, pxReshare)
	log.ErrFatal(err)
	log.Infof("New OCS created with ID: %x", ocsID)

	log.Info("2.b) Get proofs of all nodes")
	proof, err := cl.GetProofs(*roster, ocsID)
	log.ErrFatal(err)
	log.ErrFatal(proof.Verify())
	log.Info("Proof got verified successfully on nodes:")
	for i, sig := range proof.Signatures {
		log.Infof("Signature %d of %s: %x", i, proof.Roster.List[i].Address, sig)
	}

	log.Info("3.a) Creating secret key and encrypting it with the OCS-key")
	secret := []byte("ocs for everybody")
	X, err := ocsID.X()
	log.ErrFatal(err)
	U, C, err := ocs.EncodeKey(cothority.Suite, X, secret)
	log.ErrFatal(err)

	log.Info("3.b) Creating 2 certificates for the re-encryption")
	ephemeralKeyPair := key.NewKeyPair(cothority.Suite)
	wid, err := ocs.NewWriteID(X, U)
	log.ErrFatal(err)
	reencryptCert1, err := ocs.CreateCertReencrypt(caReenc1.Certificate, caReenc1.Private,
		wid, ephemeralKeyPair.Public)
	log.ErrFatal(err)
	reencryptCert2, err := ocs.CreateCertReencrypt(caReenc2.Certificate, caReenc2.Private,
		wid, ephemeralKeyPair.Public)
	log.ErrFatal(err)

	log.Info("4. Asking OCS to re-encrypt the secret to an ephemeral key")
	authRe := ocs.AuthReencrypt{
		Ephemeral: ephemeralKeyPair.Public,
		X509Cert: &ocs.AuthReencryptX509Cert{
			U:            U,
			Certificates: [][]byte{reencryptCert1.Raw, reencryptCert2.Raw},
		},
	}
	XhatEnc, err := cl.Reencrypt(*roster, ocsID, authRe)
	log.ErrFatal(err)

	log.Info("5. Decrypt the symmetric key")
	secretRec, err := ocs.DecodeKey(cothority.Suite, X, C, XhatEnc, ephemeralKeyPair.Private)
	log.ErrFatal(err)
	if bytes.Compare(secret, secretRec) != 0 {
		log.Fatal("Recovered secret is not the same")
	}

	log.Info("Successfully re-encrypted the key")
}

func printSamples() {
	s := cothority.Suite.Scalar().SetInt64(1)
	p := cothority.Suite.Point().Base()
	printScalar("* A scalar of '1':", s)
	printPoint("* The base point:", p)
	printScalar("* A scalar of '2':", s.Add(s, s))
	printPoint("* The base point added to himself:", p.Add(p, p))
	printPoint("* 2 x base:", p.Mul(s, nil))
	var allF0 [32]byte
	for i := range allF0 {
		allF0[i] = 0xf0
	}
	s.SetBytes(allF0[:])
	printScalar("* A reduced all-F0 scalar:", s)
	printScalar("* A reduced all-F0 scalar added to itself:", s.Add(s, s))
}

func bigEndianToDecimal(buf []byte) *big.Int {
	bi := &big.Int{}
	bi.SetBytes(buf)
	return bi
}

func LEBytesToDecimal(buf []byte) *big.Int {
	if len(buf)%2 != 0 {
		log.Fatal("can only convert even length slices")
	}
	for i := 0; i < len(buf)/2; i++ {
		buf[i], buf[len(buf)-i-1] = buf[len(buf)-i-1], buf[i]
	}
	return bigEndianToDecimal(buf)
}

func printScalar(msg string, s kyber.Scalar) {
	buf, err := s.MarshalBinary()
	log.ErrFatal(err)
	var str []string
	str = append(str, fmt.Sprint("Representation of a scalar:"))
	str = append(str, fmt.Sprintf("\tLittle-endian: %x", buf))
	str = append(str, fmt.Sprintf("\tDecimal: %s", LEBytesToDecimal(buf).String()))
	log.Info(msg, strings.Join(str, "\n"))
}

func printPoint(msg string, p kyber.Point) {
	ped := p.(*edwards25519.Point)
	var str []string
	str = append(str, fmt.Sprint("Representations of a point:"))
	str = append(str, fmt.Sprintf("\tCompressed: %s", ped.String()))
	str = append(str, fmt.Sprintf("\tLittle-endian X / Y:\n\t\tX: %x\n\t\tY: %x", ped.X_LE(), ped.Y_LE()))
	str = append(str, fmt.Sprintf("\tDecimal X / Y:\n\t\tX: %s\n\t\tY: %s",
		LEBytesToDecimal(ped.X_LE()).String(),
		LEBytesToDecimal(ped.Y_LE()).String()))
	log.Info(msg, strings.Join(str, "\n"))
}

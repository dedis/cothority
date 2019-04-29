// Demo of how the new OCS service works from an outside, non-go-test caller. It does the following steps:
//  1. set up a root CA that is stored in the service as being allowed to create new OCS-instances
//  2. Create a new OCS-instance with a reencryption policy being set by a node-certificate
//  3. Encrypt a symmetric key to the OCS-instance public key
//  4. Ask the OCS-instance to re-encrypt the key to an ephemeral key
//  5. Decrypt the symmetric key
package main

import (
	"bytes"
	"os"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/ocs"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3/log"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please give a roster.toml as first parameter")
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

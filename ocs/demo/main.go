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

	"go.dedis.ch/cothority/v3/ocs/certs"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/ocs"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3/log"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Please give a roster.toml as first parameter")
	}
	roster, err := lib.ReadRoster(os.Args[1])
	log.ErrFatal(err)

	log.Info("1. Creating createOCS cert and setting OCS-create policy")
	cl := ocs.NewClient()
	coPrivKey, coCert, err := certs.CreateCertCa()
	log.ErrFatal(err)
	for _, si := range roster.List {
		err = cl.AddPolicyCreateOCS(si, ocs.Policy{X509Cert: &ocs.PolicyX509Cert{
			CA: [][]byte{coCert.Raw},
		}})
		log.ErrFatal(err)
	}

	log.Info("2.a) Creating node cert")
	nodePrivKey, nodeCert, err := certs.CreateCertNode(coCert, coPrivKey)
	log.ErrFatal(err)

	px := ocs.Policy{
		X509Cert: &ocs.PolicyX509Cert{
			CA:        [][]byte{nodeCert.Raw},
			Threshold: 1,
		},
	}

	log.Info("2.b) Creating new OCS")
	oid, err := cl.CreateOCS(*roster, px, px)
	log.ErrFatal(err)
	log.Infof("New OCS created with ID: %x", oid)

	log.Info("2.c) Get proofs of all nodes")
	proof, err := cl.GetProofs(*roster, oid)
	log.ErrFatal(err)
	log.ErrFatal(proof.Verify())
	log.Info("Proof got verified successfully on nodes:")
	for i, sig := range proof.Signatures {
		log.Infof("Signature %d of %s: %x", i, proof.Roster.List[i].Address, sig)
	}

	log.Info("3.a) Creating secret key and encrypting it with the OCS-key")
	secret := []byte("ocs for everybody")
	X, err := oid.X()
	log.ErrFatal(err)
	U, C, err := certs.EncodeKey(cothority.Suite, X, secret)
	log.ErrFatal(err)

	log.Info("3.b) Creating certificate for the re-encryption")
	kp := key.NewKeyPair(cothority.Suite)
	wid, err := certs.NewWriteID(X, U)
	log.ErrFatal(err)
	reencryptCert, err := certs.CreateCertReencrypt(nodeCert, nodePrivKey, wid, kp.Public)
	log.ErrFatal(err)
	auth := ocs.AuthReencrypt{
		Ephemeral: kp.Public,
		X509Cert: &ocs.AuthReencryptX509Cert{
			U:            U,
			Certificates: [][]byte{reencryptCert.Raw},
		},
	}

	log.Info("4. Asking OCS to re-encrypt the secret to an ephemeral key")
	XhatEnc, err := cl.Reencrypt(*roster, oid, auth)
	log.ErrFatal(err)

	log.Info("5. Decrypt the symmetric key")
	secretRec, err := certs.DecodeKey(cothority.Suite, X, C, XhatEnc, kp.Private)
	log.ErrFatal(err)
	if bytes.Compare(secret, secretRec) != 0 {
		log.Fatal("Recovered secret is not the same")
	}

	log.Info("Successfully re-encrypted the key")
}

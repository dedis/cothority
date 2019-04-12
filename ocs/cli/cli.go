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
	if len(os.Args) != 2 {
		log.Fatal("Please give a roster.toml as first parameter")
	}
	roster, err := lib.ReadRoster(os.Args[1])
	log.ErrFatal(err)

	log.Info("Creating local certs")
	caPrivKey, caCert, err := ocs.CreateCaCert()
	log.ErrFatal(err)
	log.Lvl5(caPrivKey)

	px := ocs.Policy{
		X509Cert: &ocs.PolicyX509Cert{
			CA:        [][]byte{caCert.Raw},
			Threshold: 1,
		},
	}

	log.Info("Creating new OCS")
	cl := ocs.NewClient()
	var oid ocs.OCSID
	for i := 0; i < 10; i++ {
		oid, err = cl.CreateOCS(*roster, px, px)
		log.ErrFatal(err)
	}
	log.Infof("New OCS created with ID: %x", oid)

	log.Info("Creating secret key and encrypting it with the OCS-key")
	secret := []byte("ocs for everybody")
	X, err := oid.X()
	log.ErrFatal(err)
	U, C, err := ocs.EncodeKey(cothority.Suite, X, secret)
	log.ErrFatal(err)

	log.Info("Creating certificate for the re-encryption")
	kp := key.NewKeyPair(cothority.Suite)
	wid, err := ocs.NewWriteID(X, U)
	log.ErrFatal(err)
	reencryptCert, err := ocs.CreateReencryptCert(caCert, caPrivKey, wid, kp.Public)
	log.ErrFatal(err)
	auth := ocs.AuthReencrypt{
		Ephemeral: kp.Public,
		X509Cert: &ocs.AuthReencryptX509Cert{
			U:            U,
			Certificates: [][]byte{reencryptCert.Raw},
		},
	}

	log.Info("Asking OCS to re-encrypt the secret to an ephemeral key")
	XhatEnc, err := cl.Reencrypt(*roster, oid, auth)
	log.ErrFatal(err)
	secretRec, err := ocs.DecodeKey(cothority.Suite, X, C, XhatEnc, kp.Private)
	log.ErrFatal(err)
	if bytes.Compare(secret, secretRec) != 0 {
		log.Fatal("Recovered secret is not the same")
	}

	log.Info("Successfully re-encrypted the key")
}

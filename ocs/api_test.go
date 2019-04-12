package ocs

import (
	"testing"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/util/key"

	"go.dedis.ch/onet/v3/log"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/onet/v3"
)

// Creates an OCS and checks that all nodes have the same view of the OCS.
func TestClient_GetProofs(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	nbrNodes := 5
	_, roster, _ := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)

	_, caCert, err := CreateCaCert()
	require.NoError(t, err)

	px := Policy{
		X509Cert: &PolicyX509Cert{
			CA:        [][]byte{caCert.Raw},
			Threshold: 1,
		},
	}

	cl := NewClient()
	oid, err := cl.CreateOCS(*roster, px, px)
	require.NoError(t, err)

	op, err := cl.GetProofs(*roster, oid)
	require.NoError(t, op.Verify())
	require.Equal(t, len(op.Signatures), len(roster.List))
}

// Asks OCS for a reencryption of a secret
func TestClient_Reencrypt(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	nbrNodes := 5
	_, roster, _ := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)

	caPrivKey, caCert, err := CreateCaCert()
	require.NoError(t, err)
	log.Lvl5(caPrivKey)

	px := Policy{
		X509Cert: &PolicyX509Cert{
			CA:        [][]byte{caCert.Raw},
			Threshold: 1,
		},
	}

	cl := NewClient()
	var oid OCSID
	for i := 0; i < 10; i++ {
		oid, err = cl.CreateOCS(*roster, px, px)
		require.NoError(t, err)
	}

	secret := []byte("ocs for everybody")
	X, err := oid.X()
	require.NoError(t, err)
	U, C, err := EncodeKey(cothority.Suite, X, secret)
	require.NoError(t, err)

	kp := key.NewKeyPair(cothority.Suite)
	wid, err := NewWriteID(X, U)
	require.NoError(t, err)
	reencryptCert, err := CreateReencryptCert(caCert, caPrivKey, wid, kp.Public)
	require.NoError(t, err)
	auth := AuthReencrypt{
		Ephemeral: kp.Public,
		X509Cert: &AuthReencryptX509Cert{
			U:            U,
			Certificates: [][]byte{reencryptCert.Raw},
		},
	}
	for i := 0; i < 10; i++ {
		XhatEnc, err := cl.Reencrypt(*roster, oid, auth)
		require.NoError(t, err)
		secretRec, err := DecodeKey(cothority.Suite, X, C, XhatEnc, kp.Private)
		require.NoError(t, err)
		require.Equal(t, secret, secretRec)
	}
}

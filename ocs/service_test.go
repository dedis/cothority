package ocs

import (
	"testing"

	"go.dedis.ch/kyber/v3/util/key"

	"go.dedis.ch/onet/v3/log"

	"go.dedis.ch/cothority/v3"

	"github.com/stretchr/testify/require"

	"go.dedis.ch/onet/v3"
)

func TestMain(m *testing.M) {
	log.MainTest(m, 2)
}

// Test creation of a new OCS, both with a valid and with an invalid certificate.
func TestService_CreateOCS(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	nbrNodes := 2
	servers, roster, _ := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)

	// Test setting up a new OCS with a valid X509
	s1 := servers[0].Service(ServiceName).(*Service)

	px := Policy{
		X509Cert: &PolicyX509Cert{},
	}
	co := &CreateOCS{
		Roster:          *roster,
		PolicyReencrypt: px,
		PolicyReshare:   px,
	}
	cor, err := s1.CreateOCS(co)
	require.NoError(t, err)
	require.NotNil(t, cor)
	require.NotNil(t, cor.OcsID)

	// Do the same with an invalid X509
	px.X509Cert.CA = nil
	co = &CreateOCS{
		Roster:          *roster,
		PolicyReencrypt: px,
		PolicyReshare:   px,
	}
	cor, err = s1.CreateOCS(co)
	// TODO: enable test of failing creation
	//require.Error(t, err)

	// TODO: test setting up a new OCS with ByzCoin
}

// Encrypt some data and then re-encrypt it to another public key.
func TestService_Reencrypt(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	nbrNodes := 5
	servers, roster, _ := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)

	// Test setting up a new OCS with a valid X509
	s1 := servers[0].Service(ServiceName).(*Service)

	caPrivKey, caCert, err := CreateCaCert()
	require.NoError(t, err)
	caPrivKeyAttack, caCertAttack, err := CreateCaCert()
	require.NoError(t, err)

	px := Policy{
		X509Cert: &PolicyX509Cert{
			CA:        [][]byte{caCert.Raw},
			Threshold: 1,
		},
	}
	co := &CreateOCS{
		Roster:          *roster,
		PolicyReencrypt: px,
		PolicyReshare:   px,
	}
	cor, err := s1.CreateOCS(co)
	require.NoError(t, err)

	secret := []byte("ocs for all")
	X, err := cor.OcsID.X()
	require.NoError(t, err)
	U, C, err := EncodeKey(cothority.Suite, X, secret)
	require.NoError(t, err)

	kp := key.NewKeyPair(cothority.Suite)
	wid, err := NewWriteID(X, U)
	require.NoError(t, err)
	reencryptCert, err := CreateReencryptCert(caCertAttack, caPrivKeyAttack, wid, kp.Public)
	require.NoError(t, err)
	req := &Reencrypt{
		OcsID: cor.OcsID,
		Auth: AuthReencrypt{
			Ephemeral: kp.Public,
			X509Cert: &AuthReencryptX509Cert{
				U:            U,
				Certificates: [][]byte{reencryptCert.Raw},
			},
		},
	}
	rr, err := s1.Reencrypt(req)
	require.Error(t, err)

	reencryptCert, err = CreateReencryptCert(caCert, caPrivKey, wid, kp.Public)
	require.NoError(t, err)
	req.Auth.X509Cert.Certificates = [][]byte{reencryptCert.Raw}
	rr, err = s1.Reencrypt(req)
	require.NoError(t, err)

	require.NoError(t, err)
	secretRec, err := DecodeKey(cothority.Suite, X, C, rr.XhatEnc, kp.Private)
	require.NoError(t, err)
	require.Equal(t, secret, secretRec)
}

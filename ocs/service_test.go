package ocs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
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

	cc := newCaCerts(2, 2, 2)
	cc.addPolicy(servers)

	// Test setting up a new OCS with a valid X509
	s1 := servers[0].Service(ServiceName).(*Service)

	log.Lvl1("Start with insufficient number of authentications")
	co := &CreateOCS{
		Roster:          *roster,
		Auth:            cc.authCreate(1, *roster),
		PolicyReencrypt: cc.policyReencrypt,
		PolicyReshare:   cc.policyReshare,
	}
	_, err := servers[0].Service(ServiceName).(*Service).CreateOCS(co)

	log.Lvl1("Continue with copied authentication")
	ac := cc.authCreate(1, *roster)
	co.Auth.X509Cert.Certificates = append(co.Auth.X509Cert.Certificates, ac.X509Cert.Certificates[0])
	cor, err := s1.CreateOCS(co)
	require.Error(t, err)

	log.Lvl1("Correct authentication")
	co.Auth = cc.authCreate(2, *roster)
	cor, err = s1.CreateOCS(co)
	require.NoError(t, err)
	require.NotNil(t, cor)
	require.NotNil(t, cor.OcsID)
}

// Encrypt some data and then re-encrypt it to another public key.
func TestService_Reencrypt(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	nbrNodes := 5
	servers, roster, _ := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)

	cc := newCaCerts(1, 2, 2)
	cor := cc.createOCS(servers, *roster)

	// Test setting up a new OCS with a valid X509
	s1 := servers[0].Service(ServiceName).(*Service)

	secret := []byte("ocs for all")
	X, err := cor.OcsID.X()
	require.NoError(t, err)
	U, C, err := EncodeKey(cothority.Suite, X, secret)
	require.NoError(t, err)

	kp := key.NewKeyPair(cothority.Suite)
	wid, err := NewWriteID(X, U)
	require.NoError(t, err)
	req := &Reencrypt{
		OcsID: cor.OcsID,
		Auth: AuthReencrypt{
			Ephemeral: kp.Public,
			X509Cert: &AuthReencryptX509Cert{
				U:            U,
				Certificates: cc.authReencrypt(1, wid, kp.Public),
			},
		},
	}
	rr, err := s1.Reencrypt(req)
	require.Error(t, err)

	req.Auth.X509Cert.Certificates = cc.authReencrypt(2, wid, kp.Public)
	rr, err = s1.Reencrypt(req)
	require.NoError(t, err)

	require.NoError(t, err)
	secretRec, err := DecodeKey(cothority.Suite, X, C, rr.XhatEnc, kp.Private)
	require.NoError(t, err)
	require.Equal(t, secret, secretRec)
}

type caCerts struct {
	caCreate        []BCCert
	caReencrypt     []BCCert
	caReshare       []BCCert
	policyCreate    PolicyCreate
	policyReencrypt PolicyReencrypt
	policyReshare   PolicyReshare
}

func newCaCerts(nbrCr, nbrReenc, nbrReshare int) caCerts {
	cc := caCerts{}
	var cas [][]byte
	for i := 0; i < nbrCr; i++ {
		ca := NewBCCA(fmt.Sprintf("CA-Create %d", i))
		cc.caCreate = append(cc.caCreate, ca)
		cas = append(cas, ca.Certificate.Raw)
	}
	cc.policyCreate.X509Cert = &PolicyX509Cert{CA: cas, Threshold: nbrCr}

	cas = [][]byte{}
	for i := 0; i < nbrReenc; i++ {
		ca := NewBCCA(fmt.Sprintf("CA-Reencrypt %d", i))
		cc.caReencrypt = append(cc.caReencrypt, ca)
		cas = append(cas, ca.Certificate.Raw)
	}
	cc.policyReencrypt.X509Cert = &PolicyX509Cert{CA: cas, Threshold: nbrReenc}

	cas = [][]byte{}
	for i := 0; i < nbrReshare; i++ {
		ca := NewBCCA(fmt.Sprintf("CA-Reshare %d", i))
		cc.caReshare = append(cc.caReshare, ca)
		cas = append(cas, ca.Certificate.Raw)
	}
	cc.policyReshare.X509Cert = &PolicyX509Cert{CA: cas, Threshold: nbrReshare}
	return cc
}

func (cc caCerts) addPolicy(servers []*onet.Server) {
	for _, s := range servers {
		_, err := s.Service(ServiceName).(*Service).AddPolicyCreateOCS(&AddPolicyCreateOCS{Create: cc.policyCreate})
		log.ErrFatal(err)
	}
}

func (cc caCerts) createOCS(servers []*onet.Server, roster onet.Roster) *CreateOCSReply {
	cc.addPolicy(servers)
	co := &CreateOCS{
		Roster:          roster,
		Auth:            cc.authCreate(1, roster),
		PolicyReencrypt: cc.policyReencrypt,
		PolicyReshare:   cc.policyReshare,
	}
	cor, err := servers[0].Service(ServiceName).(*Service).CreateOCS(co)
	log.ErrFatal(err)
	return cor
}

func (cc caCerts) authCreate(nbr int, r onet.Roster) (ac AuthCreate) {
	if nbr > len(cc.caCreate) {
		log.Fatal("asked for too many certificates")
	}
	acx := &AuthCreateX509Cert{}
	for _, ca := range cc.caCreate[0:nbr] {
		auth := ca.CreateOCS(cc.policyReencrypt.X509Cert, cc.policyReshare.X509Cert, r)
		acx.Certificates = append(acx.Certificates, auth.Certificate.Raw)
	}
	ac.X509Cert = acx
	return
}

func (cc caCerts) authReencrypt(nbr int, wrID []byte, ephKey kyber.Point) (certs [][]byte) {
	if nbr > len(cc.caReencrypt) {
		log.Fatal("asked for too many certificates")
	}
	for _, ca := range cc.caReencrypt[0:nbr] {
		auth := ca.Reencrypt(wrID, ephKey)
		certs = append(certs, auth.Certificate.Raw)
	}
	return
}

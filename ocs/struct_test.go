package ocs

import (
	"testing"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"

	"github.com/stretchr/testify/require"
)

// TestStruct_MultiSign makes sure that with 3 CAs and a threshold of 2, verification outputs:
//  - OK for 2 certificates signed by any two different CAs
//  - OK for 3 certificates signed by any two different CAs
//  - FALSE for 2 certificates signed by the same CA
//  - FALSE for 1 certificate
func TestStruct_MultiSign(t *testing.T) {
	l := onet.NewLocalTest(cothority.Suite)
	_, r, _ := l.GenTree(2, true)
	defer l.CloseAll()
	cc := newCaCerts(3, 3, 3)
	cc.policyCreate.X509Cert.Threshold = 2

	auth := cc.authCreate(1, *r)
	require.Error(t, cc.policyCreate.verify(auth, cc.policyReencrypt, cc.policyReshare, *r))
	auth2 := cc.authCreate(1, *r)
	auth.X509Cert.Certificates = append(auth.X509Cert.Certificates, auth2.X509Cert.Certificates[0])
	require.Error(t, cc.policyCreate.verify(auth, cc.policyReencrypt, cc.policyReshare, *r))

	auth = cc.authCreate(3, *r)
	require.NoError(t, cc.policyCreate.verify(auth, cc.policyReencrypt, cc.policyReshare, *r))
	auth.X509Cert.Certificates = auth.X509Cert.Certificates[1:]
	require.NoError(t, cc.policyCreate.verify(auth, cc.policyReencrypt, cc.policyReshare, *r))
	auth.X509Cert.Certificates = append(auth.X509Cert.Certificates, auth.X509Cert.Certificates[0])
	require.NoError(t, cc.policyCreate.verify(auth, cc.policyReencrypt, cc.policyReshare, *r))
}

package service_test

import (
	"testing"

	"gopkg.in/dedis/kyber.v2/suites"

	// We need to include the service so it is started.
	"github.com/dedis/student_18_omniledger/omniledger/service"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/onet.v2"
)

var tSuite = suites.MustFind("Ed25519")

func TestClient_GetProof(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := service.NewClient()
	csr, err := c.CreateSkipchain(roster, service.Transaction{Key: []byte{1}})
	require.Nil(t, err)

	key := []byte{1, 2, 3, 4}
	value := []byte{5, 6, 7, 8}
	_, err = c.SetKeyValue(roster, csr.Skipblock.SkipChainID(),
		service.Transaction{
			Key:   key,
			Value: value,
		})
	require.Nil(t, err)

	p, err := c.GetProof(roster, csr.Skipblock.SkipChainID(), key)
	require.Nil(t, err)
	require.Nil(t, p.Proof.Verify(csr.Skipblock))
	k, vs, err := p.Proof.KeyValue()
	require.Nil(t, err)
	require.Equal(t, k, key)
	require.Equal(t, value, vs[0])
}

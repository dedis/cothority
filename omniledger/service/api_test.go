package service

import (
	"testing"
	"time"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/onet.v2"
)

func TestClient_GetProof(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	registerDummy(servers)
	defer l.CloseAll()
	defer closeQueues(l)
	signer := darc.NewSignerEd25519(nil, nil)
	c := NewClient()
	// fail when we have no signer
	csr, err := c.CreateGenesisBlock(roster)
	require.NotNil(t, err)
	csr, err = c.CreateGenesisBlock(roster, signer)
	require.Nil(t, err)

	key := []byte{1, 2, 3, 4}
	value := []byte{5, 6, 7, 8}
	kind := []byte("dummy")
	_, err = c.SetKeyValue(roster, csr.Skipblock.SkipChainID(),
		Transaction{
			Key:   key,
			Kind:  kind,
			Value: value,
		})
	require.Nil(t, err)

	var p *GetProofResponse
	var i int
	for i = 0; i < 10; i++ {
		time.Sleep(4 * waitQueueing)
		var err error
		p, err = c.GetProof(roster, csr.Skipblock.SkipChainID(), key)
		if err != nil {
			continue
		}
		if p.Proof.InclusionProof.Match() {
			break
		}
	}
	require.NotEqual(t, 10, i, "didn't get proof in time")
	require.Nil(t, p.Proof.Verify(csr.Skipblock.SkipChainID()))
	k, vs, err := p.Proof.KeyValue()
	require.Nil(t, err)
	require.Equal(t, k, key)
	require.Equal(t, value, vs[0])
}

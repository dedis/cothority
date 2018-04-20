package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/onet.v2"
)

func TestClient_GetProof(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	defer closeQueues(l)
	c := NewClient()
	csr, err := c.CreateSkipchain(roster, Transaction{Key: []byte{1}})
	require.Nil(t, err)

	key := []byte{1, 2, 3, 4}
	value := []byte{5, 6, 7, 8}
	_, err = c.SetKeyValue(roster, csr.Skipblock.SkipChainID(),
		Transaction{
			Key:   key,
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

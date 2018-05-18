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

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{"Spawn_dummy"}, signer.Identity())
	require.Nil(t, err)

	// The darc inside it should be valid.
	d := msg.GenesisDarc
	require.Nil(t, d.Verify())

	c := NewClient()
	csr, err := c.CreateGenesisBlock(roster, msg)
	require.Nil(t, err)

	// Create a new transaction.
	value := []byte{5, 6, 7, 8}
	kind := "dummy"
	tx, err := createOneClientTx(d.GetBaseID(), kind, value, signer)
	require.Nil(t, err)
	_, err = c.SetKeyValue(roster, csr.Skipblock.SkipChainID(), tx)
	require.Nil(t, err)

	// We should have a proof of our transaction in the skipchain.
	var p *GetProofResponse
	var i int
	for i = 0; i < 10; i++ {
		time.Sleep(4 * waitQueueing)
		var err error
		p, err = c.GetProof(roster, csr.Skipblock.SkipChainID(), tx.Instructions[0].ObjectID.Slice())
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
	require.Equal(t, k, tx.Instructions[0].ObjectID.Slice())
	require.Equal(t, value, vs[0])
}

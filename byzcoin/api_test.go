package byzcoin

import (
	"sync"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/require"
)

func TestClient_GetProof(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	registerDummy(servers)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{"spawn:dummy"}, signer.Identity())
	msg.BlockInterval = 100 * time.Millisecond
	require.Nil(t, err)

	// The darc inside it should be valid.
	d := msg.GenesisDarc
	require.Nil(t, d.Verify(true))

	c, csr, err := NewLedger(msg, false)
	require.Nil(t, err)

	// Create a new transaction.
	value := []byte{5, 6, 7, 8}
	kind := "dummy"
	tx, err := createOneClientTx(d.GetBaseID(), kind, value, signer)
	require.Nil(t, err)
	_, err = c.AddTransaction(tx)
	require.Nil(t, err)

	// We should have a proof of our transaction in the skipchain.
	newID := tx.Instructions[0].Hash()
	var p *GetProofResponse
	var i int
	for i = 0; i < 10; i++ {
		time.Sleep(4 * msg.BlockInterval)
		var err error
		p, err = c.GetProof(newID)
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
	require.Equal(t, k, newID)
	require.Equal(t, value, vs[0])
}

// Create a streaming client and add blocks in the background. The client
// should receive valid blocks.
func TestClient_Streaming(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	registerDummy(servers)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{"spawn:dummy"}, signer.Identity())
	msg.BlockInterval = time.Second
	require.Nil(t, err)

	// The darc inside it should be valid.
	d := msg.GenesisDarc
	require.Nil(t, d.Verify(true))

	c, csr, err := NewLedger(msg, false)
	require.Nil(t, err)

	n := 2
	go func() {
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < n; i++ {
			value := []byte{5, 6, 7, 8}
			kind := "dummy"
			tx, err := createOneClientTx(d.GetBaseID(), kind, value, signer)
			require.Nil(t, err)
			_, err = c.AddTransaction(tx)
			require.Nil(t, err)

			// sleep for a block interval so we create multiple blocks
			time.Sleep(msg.BlockInterval)
		}
	}()

	// Start collecting transactions
	c1 := NewClientKeep(csr.Skipblock.Hash, *roster)
	var xMut sync.Mutex
	var x int
	done := make(chan bool)
	cb := func(resp StreamingResponse, err error) {
		xMut.Lock()
		defer xMut.Unlock()
		if err != nil {
			// If we already closed the done channel, then it must
			// be after we've seen n blocks.
			require.True(t, x >= n)
			return
		}

		var body DataBody
		require.NotNil(t, resp.Block)
		err = protobuf.DecodeWithConstructors(resp.Block.Payload, &body, network.DefaultConstructors(cothority.Suite))
		require.NoError(t, err)
		for _, tx := range body.TxResults {
			for _, instr := range tx.ClientTransaction.Instructions {
				require.Equal(t, instr.Spawn.ContractID, "dummy")
			}
		}
		if x == n-1 {
			// We got n blocks, so we close the done channel.
			close(done)
		}
		x++
	}

	go func() {
		err = c1.StreamTransactions(csr.Skipblock.Hash, cb)
		require.Nil(t, err)
	}()
	select {
	case <-done:
	case <-time.After(time.Duration(n)*msg.BlockInterval + time.Second):
		require.Fail(t, "should have got n transactions")
	}
	require.NoError(t, c1.Close())

	// client.Close() won't close the service if there are no more
	// transactions, so send some more to make sure the service gets an
	// error and stops its streaming go-routing.
	for i := 0; i < 2; i++ {
		value := []byte{5, 6, 7, 8}
		kind := "dummy"
		tx, err := createOneClientTx(d.GetBaseID(), kind, value, signer)
		require.Nil(t, err)
		_, err = c.AddTransactionAndWait(tx, 4)
		require.Nil(t, err)
	}
}

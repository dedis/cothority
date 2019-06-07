package byzcoin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

var testSuite = cothority.Suite

func TestCollectTx(t *testing.T) {
	nNodes := []int{2, 3, 10}
	if testing.Short() {
		nNodes = []int{2, 3}
	}

	for _, n := range nNodes {
		txs, err := testRunCollectionTxProtocol(n, 1)
		require.NoError(t, err)
		require.Equal(t, n, len(txs))
	}
}

func TestCollectTx_Empty(t *testing.T) {
	txs, err := testRunCollectionTxProtocol(4, 0)
	require.NoError(t, err)
	require.Equal(t, 0, len(txs))
}

func testRunCollectionTxProtocol(n int, max int) ([]ClientTransaction, error) {
	protoPrefix := "TestCollectTx"

	getTx := func(leader *network.ServerIdentity, roster *onet.Roster, scID skipchain.SkipBlockID, latestID skipchain.SkipBlockID, max int) []ClientTransaction {
		tx := ClientTransaction{
			Instructions: []Instruction{Instruction{}},
		}
		return []ClientTransaction{tx}
	}

	protoName := fmt.Sprintf("%s_%d", protoPrefix, n)
	_, err := onet.GlobalProtocolRegister(protoName, NewCollectTxProtocol(getTx))
	if err != nil {
		return nil, err
	}

	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	_, _, tree := local.GenBigTree(n, n, n-1, true)

	p, err := local.CreateProtocol(protoName, tree)
	if err != nil {
		return nil, err
	}

	root := p.(*CollectTxProtocol)
	root.SkipchainID = skipchain.SkipBlockID("hello")
	root.LatestID = skipchain.SkipBlockID("goodbye")
	root.MaxNumTxs = max
	err = root.Start()
	if err != nil {
		return nil, err
	}

	var txs []ClientTransaction
outer:
	for {
		select {
		case newTxs, more := <-root.TxsChan:
			if more {
				txs = append(txs, newTxs...)
			} else {
				break outer
			}
		}
	}

	return txs, nil
}

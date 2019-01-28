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
	protoPrefix := "TestCollectTx"
	getTx := func(leader *network.ServerIdentity, roster *onet.Roster, scID skipchain.SkipBlockID, latestID skipchain.SkipBlockID) []ClientTransaction {
		tx := ClientTransaction{
			Instructions: []Instruction{Instruction{}},
		}
		return []ClientTransaction{tx}
	}
	for _, n := range []int{2, 3, 10} {
		protoName := fmt.Sprintf("%s_%d", protoPrefix, n)
		_, err := onet.GlobalProtocolRegister(protoName, NewCollectTxProtocol(getTx))
		require.NoError(t, err)

		local := onet.NewLocalTest(testSuite)
		_, _, tree := local.GenBigTree(n, n, n-1, true)

		p, err := local.CreateProtocol(protoName, tree)
		require.NoError(t, err)

		root := p.(*CollectTxProtocol)
		root.SkipchainID = skipchain.SkipBlockID("hello")
		root.LatestID = skipchain.SkipBlockID("goodbye")
		require.NoError(t, root.Start())

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
		require.Equal(t, len(txs), n)
		local.CloseAll()
	}
}

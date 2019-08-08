package byzcoin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	uuid "gopkg.in/satori/go.uuid.v1"
)

var testSuite = cothority.Suite

func TestCollectTx(t *testing.T) {
	nNodes := []int{2, 3, 10}
	if testing.Short() {
		nNodes = []int{2, 3}
	}

	for _, n := range nNodes {
		txs, err := testRunCollectionTxProtocol(n, 1, 1)
		require.NoError(t, err)
		require.Equal(t, n, len(txs))
	}
}

// Test that the limit is respected.
func TestCollectTx_Empty(t *testing.T) {
	txs, err := testRunCollectionTxProtocol(4, 0, 1)
	require.NoError(t, err)
	require.Equal(t, 0, len(txs))
}

// Test that an older version will ignore the limit.
func TestCollectTx_Version(t *testing.T) {
	txs, err := testRunCollectionTxProtocol(4, 0, 0)
	require.NoError(t, err)
	require.Equal(t, 4, len(txs))
}

func testRunCollectionTxProtocol(n, max, version int) ([]ClientTransaction, error) {
	protoPrefix := "TestCollectTx"

	getTx := func(leader *network.ServerIdentity, roster *onet.Roster, scID skipchain.SkipBlockID, latestID skipchain.SkipBlockID, max int) []ClientTransaction {
		tx := ClientTransaction{
			Instructions: []Instruction{Instruction{}},
		}
		return []ClientTransaction{tx}
	}

	protoName := fmt.Sprintf("%s_%d_%d_%d", protoPrefix, n, max, version)
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
	root.version = version
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

func newSI() *network.ServerIdentity {
	id := uuid.NewV1()
	return &network.ServerIdentity{
		ID: network.ServerIdentityID(id),
	}
}

// Check the version buffer features.
func TestCollectTx_VersionBuffer(t *testing.T) {
	vb := newVersionBuffer(4)
	require.Equal(t, vb.threshold, 3)

	vb.add(newSI(), 1)
	vb.add(newSI(), 2)
	si1 := newSI()
	vb.add(si1, 3)
	vb.add(si1, 3)
	vb.add(si1, 3)

	require.False(t, vb.hasThresholdFor(3))
	require.False(t, vb.hasThresholdFor(2))
	require.True(t, vb.hasThresholdFor(1))
}

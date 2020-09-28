package byzcoin

import (
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.etcd.io/bbolt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/protobuf"
)

// Test the expected use case
func TestService_StateReplay(t *testing.T) {
	b := NewBCTest(t)
	defer b.CloseAll()

	n := 2
	for i := 0; i < n; i++ {
		tx, err := createClientTxWithTwoInstrWithCounter(b.GenesisDarc.GetBaseID(), dummyContract, []byte{}, b.Signer, uint64(i*2+1))
		require.NoError(t, err)

		_, err = b.Service().AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   b.Genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: 10,
		})
		require.NoError(t, err)
	}

	_, err := b.Service().ReplayState(b.Genesis.Hash, stdFetcher{},
		ReplayStateOptions{})
	require.NoError(t, err)
}

func tryReplayBlock(t *testing.T, s *BCTest, sbID skipchain.SkipBlockID, msg string) {
	_, err := s.Service().ReplayState(sbID, stdFetcher{},
		ReplayStateOptions{MaxBlocks: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), msg)
}

func forceStoreBlock(s *BCTest, sb *skipchain.SkipBlock) error {
	return s.Service().db().Update(func(tx *bbolt.Tx) error {
		buf, err := network.Marshal(sb)
		if err != nil {
			return err
		}
		sb.Hash = sb.CalculateHash()
		return tx.Bucket([]byte("Skipchain_skipblocks")).Put(sb.Hash, buf)
	})
}

// Test that it catches failing chains and return meaningful errors
func TestService_StateReplayFailures(t *testing.T) {
	b := NewBCTest(t)
	defer b.CloseAll()

	tx, err := createClientTxWithTwoInstrWithCounter(b.GenesisDarc.GetBaseID(), dummyContract, []byte{}, b.Signer, uint64(1))
	require.NoError(t, err)

	_, err = b.Service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   b.Genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 10,
	})
	require.NoError(t, err)

	// 1. error when fetching the genesis block
	tryReplayBlock(t, b, skipchain.SkipBlockID{},
		"failed to get the first block")

	// 2. not a genesis block for the first block
	genesis := b.Service().db().GetByID(b.Genesis.Hash)
	tryReplayBlock(t, b, genesis.ForwardLink[0].To,
		"must start from genesis block")

	// 3. bad payload
	sb := skipchain.NewSkipBlock()
	sb.Roster = b.Roster
	sb.Payload = []byte{1, 1, 1, 1, 1}
	sb.ForwardLink = []*skipchain.ForwardLink{{}}
	require.NoError(t, forceStoreBlock(b, sb))
	tryReplayBlock(t, b, sb.Hash, "Error while decoding field")

	// 4. bad data
	sb.Payload = genesis.Payload
	sb.Data = []byte{1, 1, 1, 1, 1}
	require.NoError(t, forceStoreBlock(b, sb))
	tryReplayBlock(t, b, sb.Hash, "Error while decoding field")

	// 5. non matching hash
	sb.Data = []byte{}
	sb.ForwardLink = []*skipchain.ForwardLink{{}}
	require.NoError(t, forceStoreBlock(b, sb))
	tryReplayBlock(t, b, sb.Hash, "client transaction hash does not match")

	// 6. mismatching merkle trie root
	sb = b.Service().db().GetByID(b.Genesis.SkipChainID())
	var dHead DataHeader
	require.NoError(t, protobuf.Decode(sb.Data, &dHead))
	dHead.TrieRoot = []byte{1, 2, 3}
	buf, err := protobuf.Encode(&dHead)
	require.NoError(t, err)
	sb.Data = buf
	require.NoError(t, forceStoreBlock(b, sb))
	tryReplayBlock(t, b, sb.Hash, "merkle tree root doesn't match with trie root")

	// 7. failing instruction
	sb = b.Service().db().GetByID(b.Genesis.SkipChainID())
	var dBody DataBody
	require.NoError(t, protobuf.Decode(sb.Payload, &dBody))
	dBody.TxResults = append(dBody.TxResults, TxResult{
		Accepted: true,
		ClientTransaction: ClientTransaction{
			Instructions: Instructions{Instruction{}},
		},
	})
	buf, err = protobuf.Encode(&dBody)
	require.NoError(t, err)
	sb.Payload = buf
	require.NoError(t, protobuf.Decode(sb.Data, &dHead))
	dHead.ClientTransactionHash = dBody.TxResults.Hash()
	buf, err = protobuf.Encode(&dHead)
	require.NoError(t, err)
	sb.Data = buf
	require.NoError(t, forceStoreBlock(b, sb))
	tryReplayBlock(t, b, sb.Hash, "instruction verification failed")
}

// stdFetcher is a fetcher method that outputs using the onet.log library.
type stdFetcher struct {
}

func (sf stdFetcher) LogNewBlock(sb *skipchain.SkipBlock) {
	log.Infof("Replaying block at index %d", sb.Index)
}

func (sf stdFetcher) LogWarn(sb *skipchain.SkipBlock, msg, dump string) {
	log.Info(msg, dump)
}

func (sf stdFetcher) LogAppliedBlock(sb *skipchain.SkipBlock,
	head DataHeader, body DataBody) {
	txAccepted := 0
	for _, tx := range body.TxResults {
		if tx.Accepted {
			txAccepted++
		}
	}
	t := time.Unix(head.Timestamp/1e9, 0)
	log.Infof("Got correct block from %s with %d txs, "+
		"out of which %d txs got accepted",
		t.String(), len(body.TxResults), txAccepted)
}

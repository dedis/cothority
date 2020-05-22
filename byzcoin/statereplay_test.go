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
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	n := 2
	for i := 0; i < n; i++ {
		tx, err := createClientTxWithTwoInstrWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(i*2+1))
		require.NoError(t, err)

		_, err = s.service().AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   s.genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: 10,
		})
		require.NoError(t, err)
	}

	_, err := s.service().ReplayState(s.genesis.Hash, stdFetcher{},
		ReplayStateOptions{})
	require.NoError(t, err)
}

func tryReplayBlock(t *testing.T, s *ser, sbID skipchain.SkipBlockID, msg string) {
	_, err := s.service().ReplayState(sbID, stdFetcher{},
		ReplayStateOptions{MaxBlocks: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), msg)
}

func forceStoreBlock(s *ser, sb *skipchain.SkipBlock) error {
	return s.service().db().Update(func(tx *bbolt.Tx) error {
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
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	tx, err := createClientTxWithTwoInstrWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(1))
	require.NoError(t, err)

	_, err = s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 10,
	})
	require.NoError(t, err)

	// 1. error when fetching the genesis block
	tryReplayBlock(t, s, skipchain.SkipBlockID{},
		"failed to get the first block")

	// 2. not a genesis block for the first block
	genesis := s.service().db().GetByID(s.genesis.Hash)
	tryReplayBlock(t, s, genesis.ForwardLink[0].To,
		"must start from genesis block")

	// 3. bad payload
	sb := skipchain.NewSkipBlock()
	sb.Roster = s.roster
	sb.Payload = []byte{1, 1, 1, 1, 1}
	sb.ForwardLink = []*skipchain.ForwardLink{{}}
	require.NoError(t, forceStoreBlock(s, sb))
	tryReplayBlock(t, s, sb.Hash, "Error while decoding field")

	// 4. bad data
	sb.Payload = genesis.Payload
	sb.Data = []byte{1, 1, 1, 1, 1}
	require.NoError(t, forceStoreBlock(s, sb))
	tryReplayBlock(t, s, sb.Hash, "Error while decoding field")

	// 5. non matching hash
	sb.Data = []byte{}
	sb.ForwardLink = []*skipchain.ForwardLink{{}}
	require.NoError(t, forceStoreBlock(s, sb))
	tryReplayBlock(t, s, sb.Hash, "client transaction hash does not match")

	// 6. mismatching merkle trie root
	sb = s.service().db().GetByID(s.genesis.SkipChainID())
	var dHead DataHeader
	require.NoError(t, protobuf.Decode(sb.Data, &dHead))
	dHead.TrieRoot = []byte{1, 2, 3}
	buf, err := protobuf.Encode(&dHead)
	require.NoError(t, err)
	sb.Data = buf
	require.NoError(t, forceStoreBlock(s, sb))
	tryReplayBlock(t, s, sb.Hash, "merkle tree root doesn't match with trie root")

	// 7. failing instruction
	sb = s.service().db().GetByID(s.genesis.SkipChainID())
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
	require.NoError(t, forceStoreBlock(s, sb))
	tryReplayBlock(t, s, sb.Hash, "instruction verification failed")
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

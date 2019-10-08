package byzcoin

import (
	"errors"
	"testing"

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
		require.Nil(t, err)

		_, err = s.service().AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   s.genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: 10,
		})
		require.Nil(t, err)
	}

	cb := func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		return s.service().skService().GetSingleBlock(&skipchain.GetSingleBlock{ID: sib})
	}

	st, err := s.service().ReplayState(s.genesis.Hash, s.roster, cb)
	require.NoError(t, err)
	require.Equal(t, 2, st.GetIndex())
}

func tryReplay(t *testing.T, s *ser, cb BlockFetcherFunc, msg string) {
	_, err := s.service().ReplayState(s.genesis.Hash, s.roster, cb)
	require.Error(t, err)
	require.Contains(t, err.Error(), msg)
}

// Test that it catches failing chains and return meaningful errors
func TestService_StateReplayFailures(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	tx, err := createClientTxWithTwoInstrWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(1))
	require.Nil(t, err)

	_, err = s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 10,
	})
	require.Nil(t, err)

	// 1. error when fetching the genesis block
	cb := func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		return nil, errors.New("")
	}
	tryReplay(t, s, cb, "fail to get the first block:")

	// 2. not a genesis block for the first block
	cb = func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		sb := skipchain.NewSkipBlock()
		sb.Index = 1
		sb.Roster = s.roster
		return sb, nil
	}
	tryReplay(t, s, cb, "must start from genesis block")

	// 3. error when getting the next block
	cb = func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		if !sib.Equal(s.genesis.Hash) {
			return nil, errors.New("")
		}

		return s.service().skService().GetSingleBlock(&skipchain.GetSingleBlock{ID: sib})
	}
	tryReplay(t, s, cb, "replay failed to get the next block")

	// 4. bad payload
	cb = func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		sb := skipchain.NewSkipBlock()
		sb.Roster = s.roster
		sb.Payload = []byte{1, 1, 1, 1, 1}
		sb.ForwardLink = []*skipchain.ForwardLink{&skipchain.ForwardLink{}}
		return sb, nil
	}
	tryReplay(t, s, cb, "Error while decoding field")

	// 5. bad data
	cb = func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		sb := skipchain.NewSkipBlock()
		sb.Roster = s.roster
		sb.Payload = []byte{}
		sb.Data = []byte{1, 1, 1, 1, 1}
		sb.ForwardLink = []*skipchain.ForwardLink{&skipchain.ForwardLink{}}
		return sb, nil
	}
	tryReplay(t, s, cb, "Error while decoding field")

	// 6. non matching hash
	cb = func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		sb := skipchain.NewSkipBlock()
		sb.Payload = []byte{}
		sb.ForwardLink = []*skipchain.ForwardLink{&skipchain.ForwardLink{}}
		return sb, nil
	}
	tryReplay(t, s, cb, "client transaction hash does not match")

	// 7. mismatching merkle trie root
	cb = func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		sb, err := s.service().skService().GetSingleBlock(&skipchain.GetSingleBlock{ID: sib})
		if err != nil {
			return nil, err
		}

		var dHead DataHeader
		err = protobuf.Decode(sb.Data, &dHead)
		if err != nil {
			return nil, err
		}

		dHead.TrieRoot = []byte{1, 2, 3}
		buf, err := protobuf.Encode(&dHead)
		sb.Data = buf
		return sb, nil
	}
	tryReplay(t, s, cb, "merkle tree root doesn't match with trie root")

	// 8. failing instruction
	cb = func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
		sb, err := s.service().skService().GetSingleBlock(&skipchain.GetSingleBlock{ID: sib})
		if err != nil {
			return nil, err
		}

		var dBody DataBody
		err = protobuf.Decode(sb.Payload, &dBody)
		if err != nil {
			return nil, err
		}

		dBody.TxResults = append(dBody.TxResults, TxResult{
			Accepted: true,
			ClientTransaction: ClientTransaction{
				Instructions: Instructions{Instruction{}},
			},
		})
		buf, err := protobuf.Encode(&dBody)
		sb.Payload = buf

		var dHead DataHeader
		err = protobuf.Decode(sb.Data, &dHead)
		if err != nil {
			return nil, err
		}

		dHead.ClientTransactionHash = dBody.TxResults.Hash()
		buf, err = protobuf.Encode(&dHead)
		sb.Data = buf
		return sb, nil
	}
	tryReplay(t, s, cb, "instruction verification failed")
}

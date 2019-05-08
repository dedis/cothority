package byzcoin

import (
	"bytes"
	"fmt"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/protobuf"
)

// BlockFetcherFunc is a function that takes the roster and the block ID as parameter
// and return the block or an error
type BlockFetcherFunc func(roster *onet.Roster, sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error)

func replayError(sb *skipchain.SkipBlock, msg string) error {
	return fmt.Errorf("replay failed in block at index %d with message: %s", sb.Index, msg)
}

// ReplayState builds the state changes from id to the latest block
func (s *Service) ReplayState(id skipchain.SkipBlockID, ro *onet.Roster, cb BlockFetcherFunc) (ReadOnlyStateTrie, error) {
	sb, err := cb(ro, id)
	if err != nil {
		return nil, fmt.Errorf("fail to get the first block: %s", err.Error())
	}

	if sb.Index != 0 {
		// It could be possible to start from a non-genesis block but you need to download
		// the state trie first from a conode in addition to the block
		return nil, fmt.Errorf("must start from genesis block but found index %d", sb.Index)
	}

	var sst *stagingStateTrie

	for sb != nil && len(sb.ForwardLink) > 0 {
		if sb.Payload != nil {
			var dBody DataBody
			err := protobuf.Decode(sb.Payload, &dBody)
			if err != nil {
				return nil, replayError(sb, err.Error())
			}
			var dHead DataHeader
			err = protobuf.Decode(sb.Data, &dHead)
			if err != nil {
				return nil, replayError(sb, err.Error())
			}

			if !bytes.Equal(dHead.ClientTransactionHash, dBody.TxResults.Hash()) {
				return nil, replayError(sb, "client transaction has does not match")
			}

			if sb.Index == 0 {
				nonce, err := s.loadNonceFromTxs(dBody.TxResults)
				if err != nil {
					return nil, replayError(sb, err.Error())
				}
				sst, err = newMemStagingStateTrie(nonce)
				if err != nil {
					return nil, replayError(sb, err.Error())
				}
			}

			for _, tx := range dBody.TxResults {
				if tx.Accepted {
					_, sst, err = s.processOneTx(sst, tx.ClientTransaction)
					if err != nil {
						return nil, replayError(sb, err.Error())
					}
				}
			}

			if bytes.Compare(dHead.TrieRoot, sst.GetRoot()) != 0 {
				return nil, replayError(sb, "merkle tree root doesn't match with trie root")
			}
		}

		sb, err = cb(sb.Roster, sb.ForwardLink[0].To)
		if err != nil {
			return nil, fmt.Errorf("replay failed to the next block: %s", err.Error())
		}
	}

	return sst, nil
}

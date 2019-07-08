package byzcoin

import (
	"bytes"
	"errors"
	"fmt"

	"go.dedis.ch/kyber/v3/pairing"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// BlockFetcherFunc is a function that takes the roster and the block ID as parameter
// and return the block or an error
type BlockFetcherFunc func(roster *onet.Roster, sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error)

func replayError(sb *skipchain.SkipBlock, msg string) error {
	return fmt.Errorf("replay failed in block at index %d with message: %s", sb.Index, msg)
}

// ReplayState builds the state changes from the genesis of the given skipchain ID until
// the callback returns nil or a block without forward links.
// If a client wants to replay the states until the block at index x, the callback function
// must be implemented to return nil when the next block has the index x.
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

	var st *stateTrie
	roster := onet.NewRoster(ro.List)
	if roster == nil {
		return nil, errors.New("not enough valid server identities to make a roster")
	}

	for sb != nil {
		log.Infof("Replaying block at index %d", sb.Index)

		// As the roster evolves along the chain, it may happen that a roster
		// is completly offline and then we keep learning which conode happened
		// to participate so that we can try to ask for the block even if the
		// most up-to-date roster is offline.
		roster = roster.Concat(sb.Roster.List...)

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
				return nil, replayError(sb, "client transaction hash does not match")
			}

			if sb.Index == 0 {
				nonce, err := loadNonceFromTxs(dBody.TxResults)
				if err != nil {
					return nil, replayError(sb, err.Error())
				}
				st, err = newMemStateTrie(nonce)
				if err != nil {
					return nil, replayError(sb, err.Error())
				}
			}

			sst := st.MakeStagingStateTrie()

			for _, tx := range dBody.TxResults {
				if tx.Accepted {
					var scs StateChanges
					scs, sst, err = s.processOneTx(sst, tx.ClientTransaction)
					if err != nil {
						return nil, replayError(sb, err.Error())
					}

					err = st.StoreAll(scs, sb.Index)
					if err != nil {
						return nil, replayError(sb, err.Error())
					}
				}
			}

			if !bytes.Equal(dHead.TrieRoot, sst.GetRoot()) {
				log.Lvl1("Failing block:", sb.Index)
				var body DataBody
				errDecode := protobuf.Decode(sb.Payload, &body)
				if errDecode != nil {
					log.Error("couldn't decode body:", errDecode)
				} else {
					for i, tx := range body.TxResults {
						log.Lvlf1("Transaction %d: %t", i, tx.Accepted)
						for j, ct := range tx.ClientTransaction.Instructions {
							log.Lvlf1("Instruction %d: %s", j, ct)
						}
					}
				}
				return nil, replayError(sb, "merkle tree root doesn't match with trie root")
			}

			log.Lvl2("Checking links for block", sb.Index)
			pubs := sb.Roster.ServicePublics(skipchain.ServiceName)
			for j, fl := range sb.ForwardLink {
				if fl.From == nil || fl.To == nil ||
					len(fl.From) == 0 || len(fl.To) == 0 {
					log.Warnf("Forward-link %d looks broken: %+v", j, fl)
					continue
				}
				err = fl.VerifyWithScheme(pairing.NewSuiteBn256(), pubs, sb.SignatureScheme)
				if err != nil {
					log.Errorf("Found error in forward-link: '%s' - #%d: %+v", err, j, fl)
					return nil, err
				}
			}
		}

		if len(sb.ForwardLink) > 0 {
			// The level 0 forward link must be used as we need to rebuild the global
			// states for each block.
			sb, err = cb(roster, sb.ForwardLink[0].To)
			if err != nil {
				return nil, fmt.Errorf("replay failed to get the next block: %s", err.Error())
			}
		} else {
			sb = nil
		}
	}

	return st, nil
}

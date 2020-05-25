package byzcoin

import (
	"bytes"
	"errors"
	"fmt"
	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"go.dedis.ch/kyber/v3/pairing"
	"golang.org/x/xerrors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// ReplayStateLog is an interface that can be passed to ReplayState so that
// the output of the replay can be adapted to what the user wants.
type ReplayStateLog interface {
	LogNewBlock(sb *skipchain.SkipBlock)
	LogAppliedBlock(sb *skipchain.SkipBlock, head DataHeader, body DataBody)
	LogWarn(sb *skipchain.SkipBlock, msg, dump string)
}

// ReplayStateOptions is a placeholder for all future options.
// If you add a new option, be sure to keep the empty value as default.
type ReplayStateOptions struct {
	MaxBlocks    int
	VerifyFLSig  bool
	StartingTrie trie.DB
}

func replayError(sb *skipchain.SkipBlock, err error) error {
	return cothority.ErrorOrNilSkip(err, fmt.Sprintf("replay failed in block at index %d with message", sb.Index), 2)
}

// ReplayState builds the state changes from the genesis of the given skipchain ID until
// the callback returns nil or a block without forward links.
// If a client wants to replay the states until the block at index x, the callback function
// must be implemented to return nil when the next block has the index x.
func (s *Service) ReplayState(id skipchain.SkipBlockID,
	rlog ReplayStateLog, opt ReplayStateOptions) (trie.DB, error) {
	sb := s.db().GetByID(id)
	if sb == nil {
		return nil, fmt.Errorf("failed to get the first block")
	}
	if sb.Index > 0 {
		return nil, errors.New("must start from genesis block")
	}

	// Create a memory state trie that can be thrown away, but which is much
	// faster than the disk state trie.
	var dBody DataBody
	err := protobuf.Decode(sb.Payload, &dBody)
	if err != nil {
		return nil, replayError(sb, err)
	}
	nonce, err := loadNonceFromTxs(dBody.TxResults)
	if err != nil {
		return nil, replayError(sb, err)
	}
	st, err := newMemStateTrie(nonce)
	if err != nil {
		return nil, fmt.Errorf("couldn't create memory stateTrie: %v", err)
	}

	if opt.StartingTrie != nil {
		err := st.Trie.DB().Update(func(mem trie.Bucket) error {
			return opt.StartingTrie.View(func(db trie.Bucket) error {
				return db.ForEach(func(k, v []byte) error {
					return mem.Put(k, v)
				})
			})
		})
		if err != nil {
			return nil, fmt.Errorf("couldn't copy db-trie to mem-trie: %v", err)
		}
		log.LLvl2("Getting latest block:", st.GetIndex()+1)
		rep, err := s.skService().GetSingleBlockByIndex(&skipchain.
			GetSingleBlockByIndex{
			Genesis: sb.SkipChainID(),
			Index:   st.GetIndex() + 1,
		})
		if err != nil {
			return nil, fmt.Errorf("couldn't get latest block from trie: %v",
				err)
		}
		sb = rep.SkipBlock
	}

	if opt.MaxBlocks < 0 {
		latest, err := s.db().GetLatest(sb)
		if err != nil {
			return nil, fmt.Errorf("couldn't fetch latest block: %v", err)
		}
		opt.MaxBlocks = latest.Index + 1
	}

	// Start processing the blocks
	for block := 0; block < opt.MaxBlocks; block++ {
		rlog.LogNewBlock(sb)

		if sb.Payload != nil {
			var dBody DataBody
			err := protobuf.Decode(sb.Payload, &dBody)
			if err != nil {
				return nil, replayError(sb, err)
			}
			var dHead DataHeader
			err = protobuf.Decode(sb.Data, &dHead)
			if err != nil {
				return nil, replayError(sb, err)
			}

			dBody.TxResults.SetVersion(dHead.Version)

			if !bytes.Equal(dHead.ClientTransactionHash, dBody.TxResults.Hash()) {
				return nil, replayError(sb, xerrors.New("client transaction hash does not match"))
			}

			sst := st.MakeStagingStateTrie()

			var scs StateChanges
			txAccepted := 0
			for _, tx := range dBody.TxResults {
				if tx.Accepted {
					txAccepted++
					var scsTmp StateChanges
					scsTmp, sst, err = s.processOneTx(sst, tx.ClientTransaction,
						id, dHead.Timestamp)
					if err != nil {
						return nil, replayError(sb, err)
					}

					scs = append(scs, scsTmp...)
				} else {
					_, _, err = s.processOneTx(sst, tx.ClientTransaction, id, dHead.Timestamp)
					if err == nil {
						return nil, replayError(sb, xerrors.New("refused transaction passes"))
					}
				}
			}

			if !bytes.Equal(dHead.TrieRoot, sst.GetRoot()) {
				log.Errorf("Failing block-index: %d - block-version: %d",
					sb.Index, dHead.Version)
				var body DataBody
				errDecode := protobuf.Decode(sb.Payload, &body)
				if errDecode != nil {
					log.Error("couldn't decode body:", errDecode)
				} else {
					for i, tx := range body.TxResults {
						log.Infof("Transaction %d: %t", i, tx.Accepted)
						for j, ct := range tx.ClientTransaction.Instructions {
							log.Infof("Instruction %d: %s", j, ct)
						}
					}
				}
				err = xerrors.New("merkle tree root doesn't match with trie root")
				return nil, replayError(sb, err)
			}

			log.Lvl2("Checking links for block", sb.Index)
			for j, fl := range sb.ForwardLink {
				var errStr string
				if fl.From == nil || fl.To == nil ||
					len(fl.From) == 0 || len(fl.To) == 0 {
					errStr = "is missing"
					continue
				} else if !fl.From.Equal(sb.Hash) {
					errStr = "from-field doesn't match block-hash"
				} else if sbTmp := s.db().GetByID(fl.To); sbTmp == nil {
					errStr = "to-field points to non-existing block"
				}
				if errStr != "" {
					rlog.LogWarn(sb, fmt.Sprintf(
						"bad forward-link %d/%d: %s", j, len(sb.ForwardLink),
						errStr), fmt.Sprintf("%+v", fl))
					continue
				}

				if opt.VerifyFLSig {
					pubs := sb.Roster.ServicePublics(skipchain.ServiceName)
					err = fl.VerifyWithScheme(pairing.NewSuiteBn256(), pubs, sb.SignatureScheme)
					if err != nil {
						log.Errorf("Found error in forward-link: '%s' - #%d: %+v", err, j, fl)
						return nil, xerrors.Errorf("invalid forward-link: %v", err)
					}
				}
			}

			err = st.StoreAll(scs, sb.Index, dHead.Version)
			if err != nil {
				return nil, replayError(sb, err)
			}
			rlog.LogAppliedBlock(sb, dHead, dBody)
		}

		if len(sb.ForwardLink) == 0 {
			break
		} else {
			// The level 0 forward link must be used as we need to rebuild the global
			// states for each block.
			sb = s.db().GetByID(sb.ForwardLink[0].To)
			if sb == nil {
				return nil, errors.New("replay failed to get the next block")
			}
		}
	}

	return st.DB(), nil
}

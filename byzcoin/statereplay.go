package byzcoin

import (
	"bytes"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/xerrors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/pairing"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// BlockFetcherFunc is a function that takes the roster and the block ID as parameter
// and returns the block or an error
type BlockFetcherFunc func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error)

// BlockFetcher is an interface that can be passed to ReplayStateLog so that
// the output of the replay can be adapted to what the user wants.
type BlockFetcher interface {
	BlockFetcherFunc(sid skipchain.SkipBlockID) (*skipchain.SkipBlock, error)
	LogNewBlock(sb *skipchain.SkipBlock)
	LogAppliedBlock(sb *skipchain.SkipBlock, head DataHeader, body DataBody)
	LogWarn(sb *skipchain.SkipBlock, msg, dump string)
}

func replayError(sb *skipchain.SkipBlock, err error) error {
	return cothority.ErrorOrNilSkip(err, fmt.Sprintf("replay failed in block at index %d with message", sb.Index), 2)
}

// ReplayState builds the state changes from the genesis of the given skipchain ID until
// the callback returns nil or a block without forward links.
// If a client wants to replay the states until the block at index x, the callback function
// must be implemented to return nil when the next block has the index x.
func (s *Service) ReplayState(id skipchain.SkipBlockID, ro *onet.Roster, cb BlockFetcherFunc) (ReadOnlyStateTrie, error) {
	return s.ReplayStateCont(id, cb)
}

var replayTrie = "replayTrie"

// ReplayStateDB creates a stateTrie tied to the boltdb and the bucket. Every
// change to the statetrie will be reflected in the db, allowing for saving
// and resuming replays.
func (s *Service) ReplayStateDB(db *bbolt.DB, bucket []byte,
	genesis *skipchain.SkipBlock) (int, error) {
	if len(s.stateTries) == 0 {
		s.stateTries = make(map[string]*stateTrie)
	}
	var st *stateTrie
	if genesis == nil {
		var err error
		st, err = loadStateTrie(db, bucket)
		if err != nil {
			return 0, xerrors.Errorf("couldn't load state trie: %+v", err)
		}
	} else {
		var dBody DataBody
		err := protobuf.Decode(genesis.Payload, &dBody)
		if err != nil {
			return 0, xerrors.Errorf("couldn't decode payload: %+v", err)
		}
		nonce, err := loadNonceFromTxs(dBody.TxResults)
		if err != nil {
			return 0, xerrors.Errorf("couldn't get nonce: %+v", err)
		}
		st, err = newStateTrie(db, bucket, nonce)
		if err != nil {
			return 0, xerrors.Errorf("couldn't get new state trie: %+v", err)
		}
	}
	s.stateTries[replayTrie] = st
	return st.GetIndex(), nil
}

// ReplayStateContLog builds the state changes from the genesis of the given
// skipchain ID until the callback returns nil or a block without forward
// links.
// If a client wants to replay the states until the block at index x, the
// callback function must be implemented to return nil when the next block has
// the index x.
func (s *Service) ReplayStateContLog(id skipchain.SkipBlockID,
	bf BlockFetcher) (ReadOnlyStateTrie, error) {
	sb, err := bf.BlockFetcherFunc(id)
	if err != nil {
		return nil, xerrors.Errorf("fail to get the first block: %v", err)
	}

	var st *stateTrie
	if len(s.stateTries) > 0 {
		st = s.stateTries[replayTrie]
	}
	if st == nil {
		if sb.Index != 0 {
			// It could be possible to start from a non-genesis block but you need to download
			// the state trie first from a conode in addition to the block
			return nil, xerrors.Errorf("must start from genesis block but found index %d", sb.Index)
		}
	} else if st.GetIndex()+1 != sb.Index {
		return nil, xerrors.Errorf("got a skipblock with index %d for trie with"+
			" index %d", sb.Index, st.GetIndex()+1)
	}

	for sb != nil {
		bf.LogNewBlock(sb)

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

			if sb.Index == 0 && st == nil {
				nonce, err := loadNonceFromTxs(dBody.TxResults)
				if err != nil {
					return nil, replayError(sb, err)
				}
				st, err = newMemStateTrie(nonce)
				if err != nil {
					return nil, replayError(sb, err)
				}
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
			pubs := sb.Roster.ServicePublics(skipchain.ServiceName)
			for j, fl := range sb.ForwardLink {
				if fl.From == nil || fl.To == nil ||
					len(fl.From) == 0 || len(fl.To) == 0 {
					bf.LogWarn(sb,
						fmt.Sprintf("Forward-link %d looks broken", j),
						fmt.Sprintf("%+v", fl))
					continue
				}
				err = fl.VerifyWithScheme(pairing.NewSuiteBn256(), pubs, sb.SignatureScheme)
				if err != nil {
					log.Errorf("Found error in forward-link: '%s' - #%d: %+v", err, j, fl)
					return nil, xerrors.Errorf("invalid forward-link: %v", err)
				}
			}

			err = st.StoreAll(scs, sb.Index, dHead.Version)
			if err != nil {
				return nil, replayError(sb, err)
			}
			bf.LogAppliedBlock(sb, dHead, dBody)
		}

		if len(sb.ForwardLink) > 0 {
			// The level 0 forward link must be used as we need to rebuild the global
			// states for each block.
			sb, err = bf.BlockFetcherFunc(sb.ForwardLink[0].To)
			if err != nil {
				return nil, xerrors.Errorf("replay failed to get the next block: %v", err)
			}
		} else {
			sb = nil
		}
	}

	return st, nil
}

// ReplayStateCont is a wrapper over ReplayStateContLog and outputs every
// block to the std-output.
func (s *Service) ReplayStateCont(id skipchain.SkipBlockID,
	cb BlockFetcherFunc) (ReadOnlyStateTrie, error) {
	return s.ReplayStateContLog(id, stdFetcher{cb})
}

// stdFetcher is a fetcher method that outputs using the onet.log library.
type stdFetcher struct {
	cb BlockFetcherFunc
}

func (sf stdFetcher) BlockFetcherFunc(sib skipchain.SkipBlockID) (*skipchain.
	SkipBlock, error) {
	return sf.cb(sib)
}

func (sf stdFetcher) LogNewBlock(sb *skipchain.SkipBlock) {
	log.Infof("Replaying block at index %d", sb.Index)
}

func (sf stdFetcher) LogWarn(sb *skipchain.SkipBlock, msg, dump string) {
	log.Infof(msg, dump)
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

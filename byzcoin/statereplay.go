package byzcoin

import (
	"bytes"
	"fmt"
	"go.etcd.io/bbolt"
	"golang.org/x/xerrors"
	"time"

	"go.dedis.ch/kyber/v3/pairing"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// BlockFetcherFunc is a function that takes the roster and the block ID as parameter
// and returns the block or an error
type BlockFetcherFunc func(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error)

func replayError(sb *skipchain.SkipBlock, msg string) error {
	return fmt.Errorf("replay failed in block at index %d with message: %s", sb.Index, msg)
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

// ReplayStateCont builds the state changes from the genesis of the given
// skipchain ID until the callback returns nil or a block without forward
// links.
// If a client wants to replay the states until the block at index x, the
// callback function must be implemented to return nil when the next block has
// the index x.
func (s *Service) ReplayStateCont(id skipchain.SkipBlockID, cb BlockFetcherFunc) (ReadOnlyStateTrie, error) {
	sb, err := cb(id)
	if err != nil {
		return nil, fmt.Errorf("fail to get the first block: %s", err.Error())
	}

	var st *stateTrie
	if len(s.stateTries) > 0 {
		st = s.stateTries[replayTrie]
	}
	if st == nil {
		if sb.Index != 0 {
			// It could be possible to start from a non-genesis block but you need to download
			// the state trie first from a conode in addition to the block
			return nil, fmt.Errorf("must start from genesis block but found index %d", sb.Index)
		}
	} else if st.GetIndex()+1 != sb.Index {
		return nil, fmt.Errorf("got a skipblock with index %d for trie with"+
			" index %d", sb.Index, st.GetIndex()+1)
	}

	for sb != nil {
		log.Infof("Replaying block at index %d", sb.Index)

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

			dBody.TxResults.SetVersion(dHead.Version)

			if !bytes.Equal(dHead.ClientTransactionHash, dBody.TxResults.Hash()) {
				return nil, replayError(sb, "client transaction hash does not match")
			}

			if sb.Index == 0 && st == nil {
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

			var scs StateChanges
			txAccepted := 0
			for _, tx := range dBody.TxResults {
				if tx.Accepted {
					txAccepted++
					var scsTmp StateChanges
					scsTmp, sst, err = s.processOneTx(sst, tx.ClientTransaction,
						id)
					if err != nil {
						return nil, replayError(sb, err.Error())
					}

					scs = append(scs, scsTmp...)
				} else {
					_, _, err = s.processOneTx(sst, tx.ClientTransaction, id)
					if err == nil {
						return nil, replayError(sb,
							"refused transaction passes")
					}
				}
			}

			if !bytes.Equal(dHead.TrieRoot, sst.GetRoot()) {
				log.Infof("Failing block-index: %d - block-version: %d",
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

			err = st.StoreAll(scs, sb.Index, dHead.Version)
			if err != nil {
				return nil, replayError(sb, err.Error())
			}
			t := time.Unix(dHead.Timestamp/1e9, 0)
			log.Infof("Got correct block from %s with %d/%d txs",
				t.String(), len(dBody.TxResults), txAccepted)
		}

		if len(sb.ForwardLink) > 0 {
			// The level 0 forward link must be used as we need to rebuild the global
			// states for each block.
			sb, err = cb(sb.ForwardLink[0].To)
			if err != nil {
				return nil, fmt.Errorf("replay failed to get the next block: %s", err.Error())
			}
		} else {
			sb = nil
		}
	}

	return st, nil
}

package byzcoin

import (
	"encoding/binary"
	"errors"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/byzcoin/trie"
	"github.com/dedis/cothority/darc"
)

var errKeyNotSet = errors.New("key not set")

// ReadOnlyStateTrie is the read-only interface for StagingStateTrie and
// StateTrie.
type ReadOnlyStateTrie interface {
	GetValues(key []byte) (value []byte, contractID string, darcID darc.ID, err error)
	GetProof(key []byte) (*trie.Proof, error)
}

// StagingStateTrie is a wrapper around trie.StagingTrie that allows for use in
// byzcoin.
type StagingStateTrie struct {
	trie.StagingTrie
}

// Clone makes a copy of the staged data of the structure, the source Trie is
// not copied.
func (t *StagingStateTrie) Clone() *StagingStateTrie {
	return &StagingStateTrie{
		StagingTrie: *t.StagingTrie.Clone(),
	}
}

// StoreAll puts all the state changes and the index in the staging area.
func (t *StagingStateTrie) StoreAll(scs StateChanges) error {
	pairs := make([]trie.KVPair, len(scs))
	for i := range pairs {
		pairs[i] = &scs[i]
	}
	if err := t.StagingTrie.Batch(pairs); err != nil {
		return err
	}
	return nil
}

// GetValues returns the associated value, contract ID and darcID. An error is
// returned if the key does not exist or another issue occurs.
func (t *StagingStateTrie) GetValues(key []byte) (value []byte, contractID string, darcID darc.ID, err error) {
	var buf []byte
	buf, err = t.Get(key)
	if err != nil {
		return
	}
	if buf == nil {
		err = errKeyNotSet
		return
	}

	var vals StateChangeBody
	vals, err = decodeStateChangeBody(buf)
	if err != nil {
		return
	}

	value = vals.Value
	contractID = string(vals.ContractID)
	darcID = vals.DarcID
	return
}

// Commit commits the staged data to the source trie.
func (t *StagingStateTrie) Commit() error {
	// TODO if this is implemented, we can replace the stateChangeCache.
	return errors.New("not implemented")
}

const trieIndexKey = "trieIndexKey"

// StateTrie is a wrapper around trie.Trie that support the storage of an
// index.
type StateTrie struct {
	trie.Trie
}

// LoadStateTrie loads an existing StateTrie, an error is returned if no trie
// exists in db
func LoadStateTrie(db *bolt.DB, bucket, indexBucket []byte) (*StateTrie, error) {
	t, err := trie.LoadTrie(trie.NewDiskDB(db, bucket))
	if err != nil {
		return nil, err
	}
	return &StateTrie{
		Trie: *t,
	}, nil
}

// NewStateTrie creates a new, disk-based trie.Trie, an error is returned if
// the db already contains a trie.
func NewStateTrie(db *bolt.DB, bucket, nonce []byte) (*StateTrie, error) {
	t, err := trie.NewTrie(trie.NewDiskDB(db, bucket), nonce)
	if err != nil {
		return nil, err
	}
	return &StateTrie{
		Trie: *t,
	}, nil
}

// StoreAll stores the state changes in the Trie.
func (t *StateTrie) StoreAll(scs StateChanges, index int) error {
	pairs := make([]trie.KVPair, len(scs))
	for i := range pairs {
		pairs[i] = &scs[i]
	}
	return t.DB().Update(func(b trie.Bucket) error {
		if err := t.BatchWithBucket(pairs, b); err != nil {
			return err
		}
		indexBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(indexBuf, uint32(index))
		return t.SetMetadataWithBucket([]byte(trieIndexKey), indexBuf, b)
	})
}

// GetValues returns the associated value, contractID and darcID. An error is
// returned if the key does not exist.
func (t *StateTrie) GetValues(key []byte) (value []byte, contractID string, darcID darc.ID, err error) {
	var buf []byte
	buf, err = t.Get(key)
	if err != nil {
		return
	}
	if buf == nil {
		err = errKeyNotSet
		return
	}

	var vals StateChangeBody
	vals, err = decodeStateChangeBody(buf)
	if err != nil {
		return
	}

	value = vals.Value
	contractID = string(vals.ContractID)
	darcID = vals.DarcID
	return
}

// GetIndex gets the latest index.
func (t *StateTrie) GetIndex() int {
	indexBuf := t.GetMetadata([]byte(trieIndexKey))
	if indexBuf == nil {
		return -1
	}
	return int(binary.LittleEndian.Uint32(indexBuf))
}

// MakeStagingStateTrie creates a StagingStateTrie from the StateTrie.
func (t *StateTrie) MakeStagingStateTrie() *StagingStateTrie {
	return &StagingStateTrie{
		StagingTrie: *t.MakeStagingTrie(),
	}
}

// NewMemStagingStateTrie creates an in-memory StagingStateTrie.
func NewMemStagingStateTrie(nonce []byte) (*StagingStateTrie, error) {
	memTrie, err := trie.NewTrie(trie.NewMemDB(), nonce)
	if err != nil {
		return nil, err
	}
	et := StagingStateTrie{
		StagingTrie: *memTrie.MakeStagingTrie(),
	}
	return &et, nil
}

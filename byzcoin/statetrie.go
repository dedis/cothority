package byzcoin

import (
	"bytes"
	"encoding/binary"
	"errors"

	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.etcd.io/bbolt"
)

var errKeyNotSet = errors.New("key not set")

// GlobalState is used to query for any data in byzcoin.
type GlobalState interface {
	ReadOnlyStateTrie
	ReadOnlySkipChain
}

// ReadOnlyStateTrie is the read-only interface for StagingStateTrie and
// StateTrie.
type ReadOnlyStateTrie interface {
	// GetValues gets all the values associated with the given key.
	GetValues(key []byte) (value []byte, version uint64, contractID string, darcID darc.ID, err error)
	// GetProof produces an existance or absence proof for the given key.
	GetProof(key []byte) (*trie.Proof, error)
	// GetIndex returns the index metadata.
	GetIndex() int
	// GetNonce returns the nonce of the trie.
	GetNonce() ([]byte, error)
	// GetVersion returns the version of the ByzCoin protocol.
	GetVersion() Version
	// ForEach calls the callback function on every key/value pair in the
	// trie, which does not include the metadata.
	ForEach(func(k, v []byte) error) error
	// StoreAllToReplica creates a copy of the read-only trie and applies
	// the state changes to the copy. The implementation should make sure
	// that the original read-only trie is not be modified.
	StoreAllToReplica(StateChanges) (ReadOnlyStateTrie, error)
}

// ReadOnlySkipChain holds the skipchain data.
type ReadOnlySkipChain interface {
	GetLatest() (*skipchain.SkipBlock, error)
	GetGenesisBlock() (*skipchain.SkipBlock, error)
	GetBlock(skipchain.SkipBlockID) (*skipchain.SkipBlock, error)
	GetBlockByIndex(idx int) (*skipchain.SkipBlock, error)
}

type globalState struct {
	ReadOnlyStateTrie
	ReadOnlySkipChain
}

var _ GlobalState = (*globalState)(nil)

// stagingStateTrie is a wrapper around trie.StagingTrie that allows for use in
// byzcoin.
type stagingStateTrie struct {
	trie.StagingTrie
}

// Clone makes a copy of the staged data of the structure, the source Trie is
// not copied.
func (t *stagingStateTrie) Clone() *stagingStateTrie {
	return &stagingStateTrie{
		StagingTrie: *t.StagingTrie.Clone(),
	}
}

// StoreAll puts all the state changes and the index in the staging area.
func (t *stagingStateTrie) StoreAll(scs StateChanges) error {
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
func (t *stagingStateTrie) GetValues(key []byte) (value []byte, version uint64, contractID string, darcID darc.ID, err error) {
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
	version = vals.Version
	contractID = string(vals.ContractID)
	darcID = vals.DarcID
	return
}

// Commit commits the staged data to the source trie.
func (t *stagingStateTrie) Commit() error {
	return t.StagingTrie.Commit()
}

// GetIndex returns the index of the current trie.
func (t *stagingStateTrie) GetIndex() int {
	index := binary.LittleEndian.Uint32(t.StagingTrie.GetMetadata([]byte(trieIndexKey)))
	return int(index)
}

// StoreAllToReplica creates a copy of the read-only trie and applies the state
// changes to the copy.
func (t *stagingStateTrie) StoreAllToReplica(scs StateChanges) (ReadOnlyStateTrie, error) {
	newTrie := t.Clone()
	if err := newTrie.StoreAll(scs); err != nil {
		return nil, errors.New("replica failed to store state changes: " + err.Error())
	}
	return newTrie, nil
}

// GetVersion returns the version of the ByzCoin protocol.
func (t *stagingStateTrie) GetVersion() Version {
	return readVersion(t)
}

const trieIndexKey = "trieIndexKey"
const trieVersionKey = "trieVersionKey"

// stateTrie is a wrapper around trie.Trie that support the storage of an
// index.
type stateTrie struct {
	trie.Trie
}

// loadStateTrie loads an existing StateTrie, an error is returned if no trie
// exists in db
func loadStateTrie(db *bbolt.DB, bucket []byte) (*stateTrie, error) {
	t, err := trie.LoadTrie(trie.NewDiskDB(db, bucket))
	if err != nil {
		return nil, err
	}
	return &stateTrie{
		Trie: *t,
	}, nil
}

// newStateTrie creates a new, disk-based trie.Trie, an error is returned if
// the db already contains a trie.
func newStateTrie(db *bbolt.DB, bucket, nonce []byte) (*stateTrie, error) {
	t, err := trie.NewTrie(trie.NewDiskDB(db, bucket), nonce)
	if err != nil {
		return nil, err
	}
	return &stateTrie{
		Trie: *t,
	}, nil
}

// StoreAll stores the state changes in the Trie.
func (t *stateTrie) StoreAll(scs StateChanges, index int, version Version) error {
	return t.VerifiedStoreAll(scs, index, version, nil)
}

// VerifiedStoreAll stores the state changes, the index and the version as metadata. It
// checks whether the expectedRoot hash matches the computed root hash and returns an
// error if it doesn't.
func (t *stateTrie) VerifiedStoreAll(scs StateChanges, index int, version Version, expectedRoot []byte) error {
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
		if err := t.SetMetadataWithBucket([]byte(trieIndexKey), indexBuf, b); err != nil {
			return err
		}

		versionBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(versionBuf, uint32(version))
		if err := t.SetMetadataWithBucket([]byte(trieVersionKey), versionBuf, b); err != nil {
			return err
		}

		if expectedRoot != nil && !bytes.Equal(t.GetRootWithBucket(b), expectedRoot) {
			return errors.New("root verfication failed")
		}
		return nil
	})
}

// GetValues returns the associated value, contractID and darcID. An error is
// returned if the key does not exist.
func (t *stateTrie) GetValues(key []byte) (value []byte, version uint64, contractID string, darcID darc.ID, err error) {
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
	version = vals.Version
	contractID = string(vals.ContractID)
	darcID = vals.DarcID
	return
}

// GetIndex gets the latest index.
func (t *stateTrie) GetIndex() int {
	indexBuf := t.GetMetadata([]byte(trieIndexKey))
	if indexBuf == nil {
		return -1
	}
	return int(binary.LittleEndian.Uint32(indexBuf))
}

// GetVersion returns the version of the ByzCoin proocol.
func (t *stateTrie) GetVersion() Version {
	return readVersion(t)
}

// MakeStagingStateTrie creates a StagingStateTrie from the StateTrie.
func (t *stateTrie) MakeStagingStateTrie() *stagingStateTrie {
	return &stagingStateTrie{
		StagingTrie: *t.MakeStagingTrie(),
	}
}

// StoreAllToReplica is not supported. It cannot be implemented in an immutable
// way because writing state changes to the replica will change the underlying
// trie since the receiver is not a stagingStateTrie. Convert it to a
// stagingStateTrie and then use StoreAllToReplica.
func (t *stateTrie) StoreAllToReplica(scs StateChanges) (ReadOnlyStateTrie, error) {
	return nil, errors.New("unsupported operation")
}

// newMemStagingStateTrie creates an in-memory StagingStateTrie.
func newMemStagingStateTrie(nonce []byte) (*stagingStateTrie, error) {
	memTrie, err := trie.NewTrie(trie.NewMemDB(), nonce)
	if err != nil {
		return nil, err
	}
	et := stagingStateTrie{
		StagingTrie: *memTrie.MakeStagingTrie(),
	}
	return &et, nil
}

// newMemStateTrie creates an in-memory StateTrie.
func newMemStateTrie(nonce []byte) (*stateTrie, error) {
	memTrie, err := trie.NewTrie(trie.NewMemDB(), nonce)
	if err != nil {
		return nil, err
	}
	st := stateTrie{
		Trie: *memTrie,
	}
	return &st, nil
}

type roSkipChain struct {
	inner      *skipchain.Service
	genesisID  skipchain.SkipBlockID
	currLatest skipchain.SkipBlockID
}

func newROSkipChain(s *skipchain.Service, genesisID skipchain.SkipBlockID) *roSkipChain {
	return &roSkipChain{inner: s, genesisID: genesisID, currLatest: genesisID}
}

func (s *roSkipChain) GetLatest() (*skipchain.SkipBlock, error) {
	sb, err := s.inner.GetDB().GetLatestByID(s.currLatest)
	if err != nil {
		return nil, err
	}
	s.currLatest = sb.CalculateHash()
	return sb, nil
}

func (s *roSkipChain) GetGenesisBlock() (*skipchain.SkipBlock, error) {
	reply, err := s.inner.GetSingleBlockByIndex(
		&skipchain.GetSingleBlockByIndex{
			Genesis: s.genesisID,
			Index:   0,
		})
	if err != nil {
		return nil, err
	}
	return reply.SkipBlock, nil
}

func (s *roSkipChain) GetBlock(id skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
	sb := s.inner.GetDB().GetByID(id)
	if sb == nil {
		return nil, errors.New("block not found")
	}
	return sb, nil
}

func (s *roSkipChain) GetBlockByIndex(idx int) (*skipchain.SkipBlock, error) {
	reply, err := s.inner.GetSingleBlockByIndex(
		&skipchain.GetSingleBlockByIndex{
			Genesis: s.genesisID,
			Index:   idx,
		})
	if err != nil {
		return nil, err
	}
	return reply.SkipBlock, nil
}

type metadataReader interface {
	GetMetadata([]byte) []byte
}

func readVersion(t metadataReader) Version {
	buf := t.GetMetadata([]byte(trieVersionKey))
	if buf == nil {
		// Early versions didn't have the protocol version stored in the
		// metadata.
		return 0
	}

	return Version(binary.LittleEndian.Uint32(buf))
}

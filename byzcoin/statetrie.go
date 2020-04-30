package byzcoin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.etcd.io/bbolt"
	"golang.org/x/xerrors"
)

var errKeyNotSet = xerrors.New("key not set")

// GlobalState is used to query for any data in byzcoin.
type GlobalState interface {
	ReadOnlyStateTrie
	ReadOnlySkipChain
	TimeReader
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
	// GetSignerCounter returns the latest counters available.
	GetSignerCounter(id darc.Identity) (uint64, error)
	// LoadConfig returns the config,
	// or a cache of it to speed up if there are many lookups.
	LoadConfig() (*ChainConfig, error)
	// LoadDarc returns a darc, or a cache of it.
	LoadDarc(id darc.ID) (*darc.Darc, error)
}

// ReadOnlySkipChain holds the skipchain data.
type ReadOnlySkipChain interface {
	GetLatest() (*skipchain.SkipBlock, error)
	GetGenesisBlock() (*skipchain.SkipBlock, error)
	GetBlock(skipchain.SkipBlockID) (*skipchain.SkipBlock, error)
	GetBlockByIndex(idx int) (*skipchain.SkipBlock, error)
}

// TimeReader is an interface allowing to access time-related information
type TimeReader interface {
	GetCurrentBlockTimestamp() int64
}

type globalState struct {
	ReadOnlyStateTrie
	ReadOnlySkipChain
	TimeReader
}

var _ GlobalState = (*globalState)(nil)

// stagingStateTrie is a wrapper around trie.StagingTrie that allows for use in
// byzcoin.
type stagingStateTrie struct {
	trie.StagingTrie
	trieCache
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
	t.invalidate()
	pairs := make([]trie.KVPair, len(scs))
	for i := range pairs {
		pairs[i] = &scs[i]
	}
	if err := t.StagingTrie.Batch(pairs); err != nil {
		return xerrors.Errorf("batch failed: %v", err)
	}
	return nil
}

// GetValues returns the associated value, contract ID and darcID. An error is
// returned if the key does not exist or another issue occurs.
func (t *stagingStateTrie) GetValues(key []byte) (value []byte, version uint64, contractID string, darcID darc.ID, err error) {
	var buf []byte
	buf, err = t.Get(key)
	if err != nil {
		err = xerrors.Errorf("reading trie: %v", err)
		return
	}
	if buf == nil {
		err = cothority.WrapError(errKeyNotSet)
		return
	}

	var vals StateChangeBody
	vals, err = decodeStateChangeBody(buf)
	if err != nil {
		err = xerrors.Errorf("decoding body: %v", err)
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
	return cothority.ErrorOrNil(t.StagingTrie.Commit(), "commit failed")
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
		return nil, xerrors.Errorf("replica failed to store state changes: %v", err)
	}
	return newTrie, nil
}

func (t *stagingStateTrie) GetSignerCounter(id darc.Identity) (uint64, error) {
	return getSignerCounter(t, id.String())
}

// GetVersion returns the version of the ByzCoin protocol.
func (t *stagingStateTrie) GetVersion() Version {
	return readVersion(t)
}

func (t *stagingStateTrie) LoadConfig() (*ChainConfig, error) {
	return t.loadConfigFromTrie(t)
}

func (t *stagingStateTrie) LoadDarc(id darc.ID) (*darc.Darc, error) {
	return t.loadDarcFromTrie(t, id)
}

type trieCache struct {
	config *ChainConfig
	darcs  map[string]*darc.Darc
	sync.Mutex
}

func (tc *trieCache) invalidate() {
	tc.Lock()
	tc.config = nil
	tc.darcs = nil
	tc.Unlock()
}

func (tc *trieCache) loadConfigFromTrie(st ReadOnlyStateTrie) (*ChainConfig, error) {
	tc.Lock()
	defer tc.Unlock()
	if tc.config != nil {
		return tc.config, nil
	}

	// Find the genesis-darc ID.
	val, _, contract, _, err := GetValueContract(st, NewInstanceID(nil).Slice())
	if err != nil {
		return nil, fmt.Errorf("reading trie: %w", err)
	}
	if contract != ContractConfigID {
		return nil, errors.New("did not get " + ContractConfigID)
	}

	tc.config = &ChainConfig{}
	err = protobuf.DecodeWithConstructors(val, tc.config,
		network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, fmt.Errorf("decoding config: %v", err)
	}

	return tc.config, nil
}

func (tc *trieCache) loadDarcFromTrie(st ReadOnlyStateTrie,
	id darc.ID) (*darc.Darc, error) {
	tc.Lock()
	defer tc.Unlock()
	if tc.darcs == nil {
		tc.darcs = make(map[string]*darc.Darc)
	} else {
		if d, ok := tc.darcs[string(id)]; ok {
			return d, nil
		}
	}
	darcBuf, _, contract, _, err := st.GetValues(id)
	if err != nil {
		return nil, fmt.Errorf("reading trie: %v", err)
	}
	tc.Unlock()
	config, err := tc.loadConfigFromTrie(st)
	tc.Lock()
	if err != nil {
		return nil, fmt.Errorf("reading trie: %v", err)
	}
	var ok bool
	for _, id := range config.DarcContractIDs {
		if contract == id {
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("the contract '%s' is not in"+
			" the set of DARC contracts", contract)
	}
	d, err := darc.NewFromProtobuf(darcBuf)
	tc.darcs[string(id)] = d
	if err != nil {
		return nil, fmt.Errorf("decoding darc: %v", err)
	}
	return d, nil
}

const trieIndexKey = "trieIndexKey"
const trieVersionKey = "trieVersionKey"

// stateTrie is a wrapper around trie.Trie that support the storage of an
// index.
type stateTrie struct {
	trie.Trie
	trieCache
}

// loadStateTrie loads an existing StateTrie, an error is returned if no trie
// exists in db
func loadStateTrie(db *bbolt.DB, bucket []byte) (*stateTrie, error) {
	t, err := trie.LoadTrie(trie.NewDiskDB(db, bucket))
	if err != nil {
		return nil, xerrors.Errorf("loading trie: %v", err)
	}
	return &stateTrie{Trie: *t}, nil
}

// newStateTrie creates a new, disk-based trie.Trie, an error is returned if
// the db already contains a trie.
func newStateTrie(db *bbolt.DB, bucket, nonce []byte) (*stateTrie, error) {
	t, err := trie.NewTrie(trie.NewDiskDB(db, bucket), nonce)
	if err != nil {
		return nil, xerrors.Errorf("creating trie: %v", err)
	}
	return &stateTrie{Trie: *t}, nil
}

// StoreAll stores the state changes in the Trie.
func (t *stateTrie) StoreAll(scs StateChanges, index int, version Version) error {
	return cothority.ErrorOrNil(t.VerifiedStoreAll(scs, index, version, nil), "store failed")
}

// VerifiedStoreAll stores the state changes, the index and the version as metadata. It
// checks whether the expectedRoot hash matches the computed root hash and returns an
// error if it doesn't.
func (t *stateTrie) VerifiedStoreAll(scs StateChanges, index int, version Version, expectedRoot []byte) error {
	t.invalidate()
	pairs := make([]trie.KVPair, len(scs))
	for i := range pairs {
		pairs[i] = &scs[i]
	}
	return t.DB().Update(func(b trie.Bucket) error {
		if err := t.BatchWithBucket(pairs, b); err != nil {
			return xerrors.Errorf("batch failed: %v", err)
		}

		indexBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(indexBuf, uint32(index))
		if err := t.SetMetadataWithBucket([]byte(trieIndexKey), indexBuf, b); err != nil {
			return xerrors.Errorf("storing index: %v", err)
		}

		versionBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(versionBuf, uint32(version))
		if err := t.SetMetadataWithBucket([]byte(trieVersionKey), versionBuf, b); err != nil {
			return xerrors.Errorf("storing version: %v", err)
		}

		if expectedRoot != nil && !bytes.Equal(t.GetRootWithBucket(b), expectedRoot) {
			return xerrors.New("root verification failed")
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
		err = xerrors.Errorf("reading trie: %v", err)
		return
	}
	if buf == nil {
		err = cothority.WrapError(errKeyNotSet)
		return
	}

	var vals StateChangeBody
	vals, err = decodeStateChangeBody(buf)
	if err != nil {
		err = xerrors.Errorf("decoding body: %v", err)
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
	return nil, xerrors.New("unsupported operation")
}

func (t *stateTrie) GetSignerCounter(id darc.Identity) (uint64, error) {
	return getSignerCounter(t, id.String())
}

func (t *stateTrie) LoadConfig() (*ChainConfig, error) {
	return t.loadConfigFromTrie(t)
}

func (t *stateTrie) LoadDarc(id darc.ID) (*darc.Darc, error) {
	return t.loadDarcFromTrie(t, id)
}

// newMemStagingStateTrie creates an in-memory StagingStateTrie.
func newMemStagingStateTrie(nonce []byte) (*stagingStateTrie, error) {
	memTrie, err := trie.NewTrie(trie.NewMemDB(), nonce)
	if err != nil {
		return nil, xerrors.Errorf("creating trie: %v", err)
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
		return nil, xerrors.Errorf("creating trie: %v", err)
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
		return nil, xerrors.Errorf("read latest: %v", err)
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
		return nil, xerrors.Errorf("reading block: %v", err)
	}
	return reply.SkipBlock, nil
}

func (s *roSkipChain) GetBlock(id skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
	sb := s.inner.GetDB().GetByID(id)
	if sb == nil {
		return nil, xerrors.New("block not found")
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
		return nil, xerrors.Errorf("reading block: %v", err)
	}
	return reply.SkipBlock, nil
}

type currentBlockInfo struct {
	timestamp int64
}

func (info *currentBlockInfo) GetCurrentBlockTimestamp() int64 {
	return info.timestamp
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

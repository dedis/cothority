// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package bevm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"go.dedis.ch/protobuf"
)

type LowLevelDb interface {
	put(key []byte, value []byte) error
	delete(key []byte) error
	getLock() *sync.RWMutex
}

// ---------------------------------------------------------------------------
// Database distributed among BEvmValue instances
type ByzDatabase struct {
	client       *byzcoin.Client
	roStateTrie  byzcoin.ReadOnlyStateTrie
	bevmIID      byzcoin.InstanceID    // ID of the associated BEvmContract instance
	stateChanges []byzcoin.StateChange // List of state changes to apply
	keys         map[string]bool       // Keeps track of existing BEvmValue instances
	lock         sync.RWMutex
}

func createKeyMap(keyList []string) map[string]bool {
	keys := make(map[string]bool)
	for _, key := range keyList {
		keys[key] = true
	}

	return keys
}

func NewClientByzDatabase(keyList []string, client *byzcoin.Client, bevmIID byzcoin.InstanceID) (*ByzDatabase, error) {
	return &ByzDatabase{
		client:  client,
		bevmIID: bevmIID,
		keys:    createKeyMap(keyList),
	}, nil
}

func NewServerByzDatabase(keyList []string, roStateTrie byzcoin.ReadOnlyStateTrie, bevmIID byzcoin.InstanceID) (*ByzDatabase, error) {
	return &ByzDatabase{
		roStateTrie: roStateTrie,
		bevmIID:     bevmIID,
		keys:        createKeyMap(keyList),
	}, nil
}

func (db *ByzDatabase) Dump() ([]byzcoin.StateChange, []string, error) {
	// The changes produced by the EVM are apparently not ordered deterministically.
	// Their order should, however, not be relevant, because each key is only affected by one change.
	// We can tehrefore sort them as we please, as long as the sort order is deterministic to make ByzCoin happy.

	// We check the hypothesis of unique keys before going further though...
	keyMap := make(map[string]bool)
	for _, s := range db.stateChanges {
		k := string(s.Key())
		if _, ok := keyMap[k]; ok {
			return nil, nil, errors.New("Internal error: the set of changes produced by the EVM is not unique on keys")
		}
	}

	// All good, let's sort by keys
	sort.SliceStable(db.stateChanges, func(i, j int) bool {
		return string(db.stateChanges[i].Key()) < string(db.stateChanges[j].Key())
	})

	var keyList []string
	for key, _ := range db.keys {
		keyList = append(keyList, key)
	}
	// This also must be sorted as Go maps traversal order is inherently non-deterministic
	sort.Strings(keyList)

	// Compute some statistics for information purposes
	nbCreate, nbUpdate, nbRemove := 0, 0, 0
	for _, s := range db.stateChanges {
		switch s.StateAction {
		case byzcoin.Create:
			nbCreate += 1
		case byzcoin.Update:
			nbUpdate += 1
		case byzcoin.Remove:
			nbRemove += 1
		default:
			return nil, nil, errors.New(fmt.Sprintf("Unknown StateChange action: %d", s.StateAction))
		}
	}
	log.LLvlf2("%d state changes (%d Create, %d Update, %d Remove), %d entries in store",
		len(db.stateChanges), nbCreate, nbUpdate, nbRemove, len(keyList))

	return db.stateChanges, keyList, nil
}

// ---------------------------------------------------------------------------
// ethdb.Database interface implementation

func (db *ByzDatabase) getValueInstanceID(key []byte) byzcoin.InstanceID {
	// The instance ID of a value instance is given by the hash of the contract instance ID and the key

	h := sha256.New()
	h.Write(db.bevmIID[:])
	h.Write(key)

	return byzcoin.NewInstanceID(h.Sum(nil))
}

// Putter
func (db *ByzDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	return db.put(key, value)
}

// Actual implementation, callable from Batch.Write()
func (db *ByzDatabase) put(key []byte, value []byte) error {
	instanceID := db.getValueInstanceID(key)
	var sc byzcoin.StateChange

	if _, ok := db.keys[string(key)]; ok {
		sc = byzcoin.NewStateChange(byzcoin.Update, instanceID,
			ContractBEvmValueID, value, nil)
	} else {
		sc = byzcoin.NewStateChange(byzcoin.Create, instanceID,
			ContractBEvmValueID, value, nil)
	}
	db.keys[string(key)] = true

	db.stateChanges = append(db.stateChanges, sc)

	return nil
}

func (db *ByzDatabase) getBEvmValue(key []byte) (value []byte, err error) {
	instID := db.getValueInstanceID(key)

	if db.roStateTrie != nil {
		// Calling from the contract

		value, _, _, _, err = db.roStateTrie.GetValues(instID[:])
		if err != nil {
			return nil, err
		}
	} else {
		// Calling from a client

		if db.client == nil {
			return nil, errors.New("Internal error: both roStateTrie and client are nil")
		}

		// Retrieve the proof of the Byzcoin instance
		proofResponse, err := db.client.GetProof(instID[:])
		if err != nil {
			return nil, err
		}

		// Validate the proof
		err = proofResponse.Proof.Verify(db.client.ID)
		if err != nil {
			return nil, err
		}

		// Extract the value from the proof
		_, value, _, _, err = proofResponse.Proof.KeyValue()
		if err != nil {
			return nil, err
		}
	}

	return
}

// Has()
func (db *ByzDatabase) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	_, err := db.getBEvmValue(key)

	return (err == nil), nil
}

// Get()
func (db *ByzDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	value, err := db.getBEvmValue(key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Deleter
func (db *ByzDatabase) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	return db.delete(key)

	return nil
}

// Actual implementation, callable from Batch.Write()
func (db *ByzDatabase) delete(key []byte) error {
	instanceID := db.getValueInstanceID(key)

	sc := byzcoin.NewStateChange(byzcoin.Remove, instanceID,
		ContractBEvmValueID, nil, nil)

	db.stateChanges = append(db.stateChanges, sc)

	delete(db.keys, string(key))

	return nil
}

// Close()
func (db *ByzDatabase) Close() {}

// NewBatch()
func (db *ByzDatabase) NewBatch() ethdb.Batch {
	return &MemBatch{db: db}
}

func (db *ByzDatabase) getLock() *sync.RWMutex {
	return &db.lock
}

// ---------------------------------------------------------------------------
// Memory database
type MemDatabase struct {
	DB   map[string][]byte
	lock sync.RWMutex
}

type KeyValues struct {
	KVs []KeyValueEntry
}

type KeyValueEntry struct {
	Key   []byte
	Value []byte
}

// Deserialize the memory database
func NewMemDatabase(data []byte) (*MemDatabase, error) {
	kvs := &KeyValues{}

	err := protobuf.Decode(data, kvs)
	if err != nil {
		log.LLvl1("error decoding data")
		return nil, err
	}

	DB := &MemDatabase{
		DB: map[string][]byte{},
	}

	for _, kv := range kvs.KVs {
		DB.DB[string(kv.Key)] = kv.Value
	}

	return DB, nil
}

// Serialize the memory database
func (db *MemDatabase) Dump() ([]byte, error) {
	kvs := &KeyValues{}

	for key, value := range db.DB {
		kvs.KVs = append(kvs.KVs, KeyValueEntry{Key: []byte(key), Value: value})
	}
	sort.Slice(kvs.KVs, func(i, j int) bool {
		return bytes.Compare(kvs.KVs[i].Key, kvs.KVs[j].Key) < 0
	})

	return protobuf.Encode(kvs)
}

// ---------------------------------------------------------------------------
// ethdb.Database interface implementation

// Putter
func (db *MemDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	log.Lvlf3("MemDatabase.Put(key=%v, value=%v)", hex.EncodeToString(key), hex.EncodeToString(value))
	defer db.lock.Unlock()

	return db.put(key, value)
}

// Actual implementation, callable from Batch.Write()
func (db *MemDatabase) put(key []byte, value []byte) error {
	db.DB[string(key)] = common.CopyBytes(value)

	return nil
}

// Has()
func (db *MemDatabase) Has(key []byte) (bool, error) {
	db.lock.RLock()
	log.Lvlf3("MemDatabase.Has(key=%v)", hex.EncodeToString(key))
	defer db.lock.RUnlock()

	_, ok := db.DB[string(key)]

	return ok, nil
}

// Get()
func (db *MemDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	log.Lvlf3("MemDatabase.Get(key=%v)", hex.EncodeToString(key))
	defer db.lock.RUnlock()

	if entry, ok := db.DB[string(key)]; ok {
		return common.CopyBytes(entry), nil
	}

	return nil, errors.New("not found")
}

// Deleter
func (db *MemDatabase) Delete(key []byte) error {
	db.lock.Lock()
	log.Lvlf3("MemDatabase.Delete(key=%v)", hex.EncodeToString(key))
	defer db.lock.Unlock()

	return db.delete(key)

	return nil
}

// Actual implementation, callable from Batch.Write()
func (db *MemDatabase) delete(key []byte) error {
	delete(db.DB, string(key))

	return nil
}

// Close()
func (db *MemDatabase) Close() {
	log.Lvl3("MemDatabase.Close()")
}

// NewBatch()
func (db *MemDatabase) NewBatch() ethdb.Batch {
	log.Lvl3("MemDatabase.NewBatch()")

	return &MemBatch{db: db}
}

func (db *MemDatabase) getLock() *sync.RWMutex {
	return &db.lock
}

// ---------------------------------------------------------------------------
// Batch database wrapper for MemDatabase
type kv struct {
	k, v []byte
	del  bool
}

type MemBatch struct {
	db     LowLevelDb
	writes []kv
	size   int
}

// ---------------------------------------------------------------------------
// ethdb.Batch interface implementation

// Putter
func (b *MemBatch) Put(key, value []byte) error {
	log.Lvlf3("MemBatch.Put(key=%v, value=%v)", hex.EncodeToString(key), hex.EncodeToString(value))

	b.writes = append(b.writes, kv{common.CopyBytes(key), common.CopyBytes(value), false})
	b.size += len(value)

	return nil
}

// Deleter
func (b *MemBatch) Delete(key []byte) error {
	log.Lvlf3("MemBatch.Delete(key=%v)", hex.EncodeToString(key))

	b.writes = append(b.writes, kv{common.CopyBytes(key), nil, true})
	b.size++

	return nil
}

// Write()
func (b *MemBatch) Write() error {
	b.db.getLock().Lock()
	log.Lvl3("MemBatch.Write()")
	defer b.db.getLock().Unlock()

	var err error

	// Apply batch commands to the underlying database
	for _, kv := range b.writes {
		if kv.del {
			err = b.db.delete(kv.k)
		} else {
			err = b.db.put(kv.k, kv.v)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// ValueSize()
func (b *MemBatch) ValueSize() int {
	log.Lvl3("MemBatch.ValueSize()")

	return b.size
}

// Reset()
func (b *MemBatch) Reset() {
	log.Lvl3("MemBatch.Reset()")

	b.writes = b.writes[:0]
	b.size = 0
}

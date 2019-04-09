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
	"encoding/hex"
	"errors"
	"sort"
	"sync"

	"go.dedis.ch/onet/v3/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"go.dedis.ch/protobuf"
)

// ---------------------------------------------------------------------------
// Memory database, adapted from Ethereum code.
// See https://github.com/ethereum/go-ethereum/blob/release/1.8/ethdb/memory_database.go

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

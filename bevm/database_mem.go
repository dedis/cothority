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
// Ethereum state memory database, adapted from Ethereum code.
// See https://github.com/ethereum/go-ethereum/blob/release/1.8/ethdb/memory_database.go

// MemDatabase main abstraction
type MemDatabase struct {
	DB   map[string][]byte
	lock sync.RWMutex
}

type keyValues struct {
	KVs []keyValueEntry
}

type keyValueEntry struct {
	Key   []byte
	Value []byte
}

// NewMemDatabase deserializes the memory database
func NewMemDatabase(data []byte) (*MemDatabase, error) {
	kvs := &keyValues{}

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

// Dump serializes the memory database
func (db *MemDatabase) Dump() ([]byte, error) {
	kvs := &keyValues{}

	for key, value := range db.DB {
		kvs.KVs = append(kvs.KVs, keyValueEntry{Key: []byte(key), Value: value})
	}
	sort.Slice(kvs.KVs, func(i, j int) bool {
		return bytes.Compare(kvs.KVs[i].Key, kvs.KVs[j].Key) < 0
	})

	return protobuf.Encode(kvs)
}

// ethdb.Database interface implementation

// Put implements Putter.Put()
func (db *MemDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	log.Lvlf3("MemDatabase.Put(key=%v, value=%v)", hex.EncodeToString(key), hex.EncodeToString(value))
	defer db.lock.Unlock()

	return db.put(key, value)
}

// Implements lowLevelDb.put()
func (db *MemDatabase) put(key []byte, value []byte) error {
	db.DB[string(key)] = common.CopyBytes(value)

	return nil
}

// Has implements Has()
func (db *MemDatabase) Has(key []byte) (bool, error) {
	db.lock.RLock()
	log.Lvlf3("MemDatabase.Has(key=%v)", hex.EncodeToString(key))
	defer db.lock.RUnlock()

	_, ok := db.DB[string(key)]

	return ok, nil
}

// Get implements Get()
func (db *MemDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	log.Lvlf3("MemDatabase.Get(key=%v)", hex.EncodeToString(key))
	defer db.lock.RUnlock()

	if entry, ok := db.DB[string(key)]; ok {
		return common.CopyBytes(entry), nil
	}

	return nil, errors.New("not found")
}

// Delete implements Deleter.Delete()
func (db *MemDatabase) Delete(key []byte) error {
	db.lock.Lock()
	log.Lvlf3("MemDatabase.Delete(key=%v)", hex.EncodeToString(key))
	defer db.lock.Unlock()

	return db.delete(key)
}

// Implements lowLevelDb.delete()
func (db *MemDatabase) delete(key []byte) error {
	delete(db.DB, string(key))

	return nil
}

// Close implements Close()
func (db *MemDatabase) Close() {
	log.Lvl3("MemDatabase.Close()")
}

// NewBatch implements NewBatch()
func (db *MemDatabase) NewBatch() ethdb.Batch {
	log.Lvl3("MemDatabase.NewBatch()")

	return &memBatch{db: db}
}

// Implements lowLevelDb.getLock()
func (db *MemDatabase) getLock() *sync.RWMutex {
	return &db.lock
}

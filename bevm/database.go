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

package byzcoin

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sort"
	"sync"

	"go.dedis.ch/onet/v3/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"go.dedis.ch/protobuf"
)

//MemDatabase structure
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

//NewMemDatabase creates a new memory database
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
	if err != nil {
		log.Lvl1("Error with memory database", err)
		return nil, err
	}
	return DB, nil
}

//NewMemDatabaseWithCap :
func NewMemDatabaseWithCap(size int) *MemDatabase {
	return &MemDatabase{
		DB: make(map[string][]byte, size),
	}
}

//Dump encodes the data back
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

//Put :
func (db *MemDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	//log.Print(key, value)
	db.DB[string(key)] = common.CopyBytes(value)
	return nil
}

//Has :
func (db *MemDatabase) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	_, ok := db.DB[string(key)]
	return ok, nil
}

//Get  :
func (db *MemDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if entry, ok := db.DB[string(key)]; ok {
		return common.CopyBytes(entry), nil
	}
	return nil, errors.New("not found")
}

//Keys :
func (db *MemDatabase) Keys() [][]byte {
	db.lock.RLock()
	defer db.lock.RUnlock()

	keys := [][]byte{}
	for key := range db.DB {
		keys = append(keys, []byte(key))
	}
	return keys
}

//Delete :
func (db *MemDatabase) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	delete(db.DB, string(key))
	return nil
}

//Close :
func (db *MemDatabase) Close() {}

//NewBatch :
func (db *MemDatabase) NewBatch() ethdb.Batch {
	return &memBatch{db: db}
}

//Len :
func (db *MemDatabase) Len() int { return len(db.DB) }

type kv struct {
	k, v []byte
	del  bool
}

type memBatch struct {
	db     *MemDatabase
	writes []kv
	size   int
}

func (b *memBatch) Put(key, value []byte) error {
	h := sha256.New()
	h.Write(key)
	h.Write(value)
	//log.Printf("%x: %x / %x", h.Sum(nil), key, value)
	b.writes = append(b.writes, kv{common.CopyBytes(key), common.CopyBytes(value), false})
	b.size += len(value)
	return nil
}

func (b *memBatch) Delete(key []byte) error {
	b.writes = append(b.writes, kv{common.CopyBytes(key), nil, true})
	b.size++
	return nil
}

func (b *memBatch) Write() error {
	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	for _, kv := range b.writes {
		h := sha256.New()
		h.Write(kv.k)
		h.Write(kv.v)
		//log.Printf("%x: %x / %x", h.Sum(nil), kv.k, kv.v)
		if kv.del {
			delete(b.db.DB, string(kv.k))
			continue
		}
		b.db.DB[string(kv.k)] = kv.v
	}
	return nil
}

func (b *memBatch) ValueSize() int {
	return b.size
}

func (b *memBatch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

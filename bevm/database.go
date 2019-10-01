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
	"encoding/hex"
	"sync"

	"go.dedis.ch/onet/v3/log"

	"github.com/ethereum/go-ethereum/common"
)

// Batch of database update operations, adapted from Ethereum code.
// See https://github.com/ethereum/go-ethereum/blob/release/1.8/ethdb/memory_database.go
type kv struct {
	k, v []byte
	del  bool
}

// Interface allowing to plus different database implementations to a memBatch
type lowLevelDb interface {
	put(key []byte, value []byte) error
	delete(key []byte) error
	getLock() *sync.RWMutex
}

type memBatch struct {
	db     lowLevelDb
	writes []kv
	size   int
}

// ethdb.Batch interface implementation

// Putter
func (b *memBatch) Put(key, value []byte) error {
	log.Lvlf3("memBatch.Put(key=%v, value=%v)", hex.EncodeToString(key), hex.EncodeToString(value))

	b.writes = append(b.writes, kv{common.CopyBytes(key), common.CopyBytes(value), false})
	b.size += len(value)

	return nil
}

// Deleter
func (b *memBatch) Delete(key []byte) error {
	log.Lvlf3("memBatch.Delete(key=%v)", hex.EncodeToString(key))

	b.writes = append(b.writes, kv{common.CopyBytes(key), nil, true})
	b.size++

	return nil
}

// Write()
func (b *memBatch) Write() error {
	b.db.getLock().Lock()
	log.Lvl3("memBatch.Write()")
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
func (b *memBatch) ValueSize() int {
	log.Lvl3("memBatch.ValueSize()")

	return b.size
}

// Reset()
func (b *memBatch) Reset() {
	log.Lvl3("memBatch.Reset()")

	b.writes = b.writes[:0]
	b.size = 0
}

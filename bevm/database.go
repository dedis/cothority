package bevm

import (
	"encoding/hex"
	"sync"

	"go.dedis.ch/onet/v3/log"

	"github.com/ethereum/go-ethereum/common"
)

type LowLevelDb interface {
	put(key []byte, value []byte) error
	delete(key []byte) error
	getLock() *sync.RWMutex
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

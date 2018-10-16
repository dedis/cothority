package trie

import (
	"errors"
	"sync"
)

// memDB is the DB implementation for an in-memory database.
type memDB struct {
	bucket *memBucket
	sync.Mutex
}

// NewMemDB creates a new in-memory database.
func NewMemDB() DB {
	bucket := newMemBucket()
	db := memDB{
		bucket: &bucket,
	}
	return &db
}

func (r *memDB) Update(f func(bucket) error) error {
	r.Lock()
	defer r.Unlock()
	return f(r.bucket)

}

func (r *memDB) View(f func(bucket) error) error {
	r.Lock()
	defer r.Unlock()
	return f(r.bucket)
}

func (r *memDB) UpdateDryRun(f func(bucket) error) error {
	return errors.New("dry-run is not currently supported for in-memory database")
}

// Close delete the memory-only database, it cannot be recovered.
func (r *memDB) Close() error {
	r.bucket = nil
	return nil
}

type memBucket struct {
	storage map[string][]byte
}

func newMemBucket() memBucket {
	return memBucket{
		storage: make(map[string][]byte),
	}
}

func (r *memBucket) Get(k []byte) []byte {
	return r.storage[string(k)]
}

func (r *memBucket) Put(k, v []byte) error {
	r.storage[string(k)] = v
	return nil
}

func (r *memBucket) Delete(k []byte) error {
	delete(r.storage, string(k))
	return nil
}

func (r memBucket) ForEach(f func(k, v []byte) error) error {
	for k, v := range r.storage {
		if err := f([]byte(k), v); err != nil {
			return err
		}
	}
	return nil
}

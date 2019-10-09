package trie

import (
	"sync"

	"golang.org/x/xerrors"
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

func (r *memDB) Update(f func(Bucket) error) error {
	r.Lock()
	defer r.Unlock()

	r.bucket.writable = true
	return f(r.bucket)

}

func (r *memDB) View(f func(Bucket) error) error {
	r.Lock()
	defer r.Unlock()

	r.bucket.writable = false
	return f(r.bucket)
}

// UpdateDryRun attempts to execute the operations on a copy of the database
// and then discards it. Hence, it may be more expensive than the UpdateDryRun
// in the disk implementation which rolls back transactions. It is useful for
// seeing the intermediate values. If they need to be used after doing the
// dry-run, they should be copied.
func (r *memDB) UpdateDryRun(f func(Bucket) error) error {
	r.Lock()
	defer r.Unlock()
	r.bucket.writable = false

	clone := r.bucket.clone()
	clone.writable = true

	return f(clone)
}

// Close delete the memory-only database, the data cannot be recovered.
func (r *memDB) Close() error {
	r.bucket = nil
	return nil
}

type memBucket struct {
	storage  map[string][]byte
	writable bool
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
	if !r.writable {
		return xerrors.New("trying to use Put in a read-only transaction")
	}
	r.storage[string(k)] = clone(v)
	return nil
}

func (r *memBucket) Delete(k []byte) error {
	if !r.writable {
		return xerrors.New("trying to use Put in a read-only transaction")
	}
	delete(r.storage, string(k))
	return nil
}

func (r *memBucket) ForEach(f func(k, v []byte) error) error {
	for k, v := range r.storage {
		if err := f([]byte(k), v); err != nil {
			return err
		}
	}
	return nil
}

func (r *memBucket) clone() *memBucket {
	clone := make(map[string][]byte)
	for k, v := range r.storage {
		clone[k] = v
	}
	return &memBucket{
		storage:  clone,
		writable: r.writable,
	}
}

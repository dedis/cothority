package trie

import "sync"

// implementation for memorydb

type memDB struct {
	tx *memTx
	sync.RWMutex
}

func NewMemDB() database {
	tx := newMemTx()
	db := memDB{
		tx: &tx,
	}
	return &db
}

func (r *memDB) Update(f func(transaction) error) error {
	r.Lock()
	defer r.Unlock()
	return f(r.tx)

}
func (r *memDB) View(f func(transaction) error) error {
	r.RLock()
	defer r.RUnlock()
	return f(r.tx)
}

// Close delete the memory-only database, it cannot be recovered.
func (r *memDB) Close() error {
	r.tx = nil
	return nil
}

type memTx struct {
	buckets map[string]*memBucket
}

func newMemTx() memTx {
	return memTx{
		buckets: make(map[string]*memBucket),
	}
}

func (r *memTx) Bucket(b []byte) bucket {
	return r.buckets[string(b)]
}

func (r *memTx) CreateBucketIfNotExists(b []byte) (bucket, error) {
	if b, ok := r.buckets[string(b)]; ok {
		return b, nil
	}
	newBucket := newMemBucket()
	r.buckets[string(b)] = &newBucket
	return &newBucket, nil
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

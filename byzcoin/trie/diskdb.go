package trie

import bolt "github.com/coreos/bbolt"

// implementation for boltdb

type diskDB struct {
	db *bolt.DB
}

func (r *diskDB) Update(f func(transaction) error) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		return f(&diskTx{tx})
	})
}

func (r *diskDB) View(f func(transaction) error) error {
	return r.db.View(func(tx *bolt.Tx) error {
		return f(&diskTx{tx})
	})
}

func (r *diskDB) Close() error {
	return r.db.Close()
}

type diskTx struct {
	tx *bolt.Tx
}

func (r *diskTx) Bucket(b []byte) bucket {
	return &diskBucket{r.tx.Bucket(b)}
}

func (r *diskTx) CreateBucketIfNotExists(b []byte) (bucket, error) {
	bucket, err := r.tx.CreateBucketIfNotExists(b)
	if err != nil {
		return nil, err
	}
	return &diskBucket{bucket}, nil
}

type diskBucket struct {
	b *bolt.Bucket
}

func (r *diskBucket) Delete(k []byte) error {
	return r.b.Delete(k)
}

func (r *diskBucket) Put(k, v []byte) error {
	return r.b.Put(k, v)
}

func (r *diskBucket) Get(k []byte) []byte {
	return r.b.Get(k)
}

func (r *diskBucket) ForEach(f func(k, v []byte) error) error {
	return r.b.ForEach(f)
}

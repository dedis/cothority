package trie

import (
	"errors"

	bolt "github.com/coreos/bbolt"
)

// implementation for boltdb

type diskDB struct {
	db     *bolt.DB
	bucket []byte
}

func (r *diskDB) Update(f func(bucket) error) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		return f(&diskBucket{b})
	})
}

func (r *diskDB) View(f func(bucket) error) error {
	return r.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(r.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		return f(&diskBucket{b})
	})
}

func (r *diskDB) Close() error {
	return r.db.Close()
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

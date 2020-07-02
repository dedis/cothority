package trie

import (
	"errors"
	"go.etcd.io/bbolt"
)

var errDryRun = errors.New("this is a dry-run")

// diskDB is the DB implementation for boltdb.
type diskDB struct {
	db     *bbolt.DB
	bucket []byte
}

// NewDiskDB creates a new boltdb-backed database.
func NewDiskDB(db *bbolt.DB, bucket []byte) DB {
	disk := diskDB{
		db:     db,
		bucket: bucket,
	}
	return &disk
}

func (r *diskDB) Update(f func(Bucket) error) error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(r.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		return f(&diskBucket{b})
	})
}

func (r *diskDB) View(f func(Bucket) error) error {
	return r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(r.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		return f(&diskBucket{b})
	})
}

// UpdateDryRun executes the given transaction and then performs a rollback at
// the end to return the database to its earlier state (before UpdateDryRun is
// called). It is useful for seeing the intermediate values. If they need to be
// used after doing the dry-run, they should be copied.
func (r *diskDB) UpdateDryRun(f func(Bucket) error) error {
	err := r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(r.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		if err := f(&diskBucket{b}); err != nil {
			return err
		}
		return errDryRun
	})
	if err != errDryRun {
		return err
	}
	return nil
}

func (r *diskDB) Close() error {
	return r.db.Close()
}

type diskBucket struct {
	b *bbolt.Bucket
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

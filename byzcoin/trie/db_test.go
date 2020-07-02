package trie

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDB(t *testing.T) {
	testMemAndDisk(t, testDB)
}

func testDB(t *testing.T, db DB) {
	// Write things in a write tx should be ok.
	err := db.Update(func(b Bucket) error {
		for i := 0; i < 10; i++ {
			k := []byte{byte(i)}
			if err := b.Put(k, k); err != nil {
				return err
			}
		}
		for i := 0; i < 10; i++ {
			k := []byte{byte(i)}
			if v := b.Get(k); !bytes.Equal(k, v) {
				return errors.New("got an unexpected value")
			}
		}
		return nil
	})
	require.NoError(t, err)

	// Read these back in a read tx should be ok.
	err = db.View(func(b Bucket) error {
		for i := 0; i < 10; i++ {
			k := []byte{byte(i)}
			if v := b.Get(k); !bytes.Equal(k, v) {
				return errors.New("got an unexpected value")
			}
		}
		return nil
	})
	require.NoError(t, err)

	// Write thing in a read tx should fail.
	err = db.View(func(b Bucket) error {
		return b.Put([]byte("hello"), []byte("world"))
	})
	require.Error(t, err)

	// The failed tx should not exist.
	err = db.View(func(b Bucket) error {
		v := b.Get([]byte("hello"))
		if v != nil {
			return errors.New("failed tx exists")
		}
		return nil
	})
	require.NoError(t, err)

	// Check that the iteration is correct.
	var cnt int
	err = db.View(func(b Bucket) error {
		return b.ForEach(func(k, v []byte) error {
			cnt++
			return nil
		})
	})
	require.NoError(t, err)
	require.Equal(t, 10, cnt)

	// Delete everything and the iteration should find nothing.
	var cntRem int
	err = db.Update(func(b Bucket) error {
		for i := 0; i < 10; i++ {
			k := []byte{byte(i)}
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return b.ForEach(func(k, v []byte) error {
			cntRem++
			return nil
		})
	})
	require.NoError(t, err)
	require.Zero(t, cntRem)
}

func TestDBDryRun(t *testing.T) {
	testMemAndDisk(t, testDB)
}

func testMemAndDisk(t *testing.T, f func(*testing.T, DB)) {
	mem := NewMemDB()
	defer mem.Close()
	f(t, mem)

	disk := newDiskDB(t)
	defer delDiskDB(t, disk)
	f(t, disk)
}

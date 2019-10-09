package trie

import (
	"bytes"
	"crypto/rand"
	"os"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"
	bbolt "go.etcd.io/bbolt"
	"golang.org/x/xerrors"
)

const testDBName = "test_trie.db"
const bucketName = "test_trie_bucket"

func TestNewTrie(t *testing.T) {
	testMemAndDisk(t, testNewTrie)
}

func testNewTrie(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	// If we iterate the database, we should only have 5 items - the root,
	// the two empty leaves, the entry point and the nonce.
	var cnt int
	db.View(func(b Bucket) error {
		return b.ForEach(func(k, v []byte) error {
			cnt++
			return nil
		})
	})
	require.Equal(t, 5, cnt)

	nonce1 := make([]byte, 32)
	copy(nonce1, testTrie.nonce)

	// Load the database again and the nonce should still be there.
	testTrie, err = LoadTrie(db)
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	require.Equal(t, testTrie.nonce, nonce1)

	// Check that the root has two children and they exist.
	root := getRootNode(t, db)
	leftBuf, err := testTrie.getRaw(root.Left)
	require.NoError(t, err)
	left, err := decodeEmptyNode(leftBuf)
	require.NoError(t, err)

	rightBuf, err := testTrie.getRaw(root.Right)
	require.NoError(t, err)
	right, err := decodeEmptyNode(rightBuf)
	require.NoError(t, err)

	require.Equal(t, root.Left, left.hash(testTrie.nonce))
	require.Equal(t, root.Right, right.hash(testTrie.nonce))

	// Check validity.
	require.NoError(t, testTrie.IsValid())
}

func TestAddToEmptyNode(t *testing.T) {
	testMemAndDisk(t, testAddToEmptyNode)
}

func testAddToEmptyNode(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	// Set two values, which should make the two empty nodes into leaf
	// nodes. 0xff -> MSB is 1, 0x7f -> MSB is 0
	require.NoError(t, testTrie.Set([]byte{0xff}, []byte{0xff}))
	require.NoError(t, testTrie.Set([]byte{0x7f}, []byte{127}))

	val0, err := testTrie.Get([]byte{0xff})
	require.NoError(t, err)
	require.Equal(t, val0, []byte{0xff})

	val1, err := testTrie.Get([]byte{0x7f})
	require.NoError(t, err)
	require.Equal(t, val1, []byte{0x7f})

	// Check validity.
	require.NoError(t, testTrie.IsValid())
}

func TestAddToLeafNode(t *testing.T) {
	testMemAndDisk(t, testAddToLeafNode)
}

func testAddToLeafNode(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	// Set two values, which should make the two empty nodes into leaf
	// nodes. 0xff is 11111111, 0xdf is 11011111. So we need to create new
	// interior nodes.
	require.NoError(t, testTrie.Set([]byte{0xff}, []byte{0xff}))
	require.NoError(t, testTrie.Set([]byte{0xdf}, []byte{0xdf}))

	val0, err := testTrie.Get([]byte{0xff})
	require.NoError(t, err)
	require.Equal(t, val0, []byte{0xff})

	val1, err := testTrie.Get([]byte{0xdf})
	require.NoError(t, err)
	require.Equal(t, val1, []byte{0xdf})

	// On the other side of the tree, we add nodes on 0x00 and 0x01 to
	// create a even longer list of interior nodes.
	require.NoError(t, testTrie.Set([]byte{0x00}, []byte{0x00}))
	require.NoError(t, testTrie.Set([]byte{0x01}, []byte{0x01}))

	val2, err := testTrie.Get([]byte{0x00})
	require.NoError(t, err)
	require.Equal(t, val2, []byte{0x00})

	val3, err := testTrie.Get([]byte{0x01})
	require.NoError(t, err)
	require.Equal(t, val3, []byte{0x01})

	// Check validity.
	require.NoError(t, testTrie.IsValid())
}

func TestLongThenShortKey(t *testing.T) {
	testMemAndDisk(t, testLongThenShortKey)
}

func testLongThenShortKey(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	// Use a long key then a short key where the short key has the same
	// prefix as the long key. We should be able to find both keys.
	longKey := []byte{0xff, 0xff, 0xff, 0xff}
	shortKey := []byte{0xff}
	require.NoError(t, testTrie.Set(longKey, longKey))
	require.NoError(t, testTrie.Set(shortKey, shortKey))

	longVal, err := testTrie.Get(longKey)
	require.NoError(t, err)
	require.Equal(t, longVal, longKey)
	shortVal, err := testTrie.Get(shortKey)
	require.NoError(t, err)
	require.Equal(t, shortVal, shortKey)

	// Check validity.
	require.NoError(t, testTrie.IsValid())
}

func TestOverwrite(t *testing.T) {
	testMemAndDisk(t, testOverwrite)
}

func testOverwrite(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	// Overwrite one value many times
	for i := 0; i < 10; i++ {
		v := []byte{byte(i)}
		require.NoError(t, testTrie.Set([]byte{0xff}, v))
		val, err := testTrie.Get([]byte{0xff})
		require.NoError(t, err)
		require.Equal(t, val, v)
	}

	// Overwrite many values once
	for i := 12; i < 50; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Set(k, k))
	}
	for i := 12; i < 50; i++ {
		k := []byte{byte(i)}
		v := []byte{byte(i + 1)}
		require.NoError(t, testTrie.Set(k, v))
	}
	for i := 12; i < 50; i++ {
		k := []byte{byte(i)}
		v := []byte{byte(i + 1)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, v)
	}

	// Check validity.
	require.NoError(t, testTrie.IsValid())
}

func TestDelete(t *testing.T) {
	testMemAndDisk(t, testDelete)
}

func testDelete(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	// Create some keys
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Set(k, k))
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Delete them
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Delete(k))
	}

	// They should disappear
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}

	// We should be allowed to delete again and nothing should happen.
	oldRoot := testTrie.GetRoot()
	require.NotNil(t, oldRoot)
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Delete(k))
	}
	require.Equal(t, oldRoot, testTrie.GetRoot())

	// Check validity.
	require.NoError(t, testTrie.IsValid())

	// TODO the following is not true because we cannot shink the tree yet
	/*
		// If we iterate the database, we should only have 5 items - the root,
		// the two empty leaves, the entry point and the nonce.
		var cnt int
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketName))
			return b.ForEach(func(k, v []byte) error {
				cnt++
				return nil
			})
		})
		require.Equal(t, 5, cnt)
	*/
}

func TestSetDeleteSet(t *testing.T) {
	testMemAndDisk(t, testSetDeleteSet)
}

func testSetDeleteSet(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	require.NoError(t, testTrie.Set([]byte{0xff}, []byte{0xff}))
	require.NoError(t, testTrie.Set([]byte{0xdf}, []byte{0xdf}))

	require.NoError(t, testTrie.Delete([]byte{0xff}))
	require.NoError(t, testTrie.Delete([]byte{0xdf}))

	require.NoError(t, testTrie.Set([]byte{0xff}, []byte{0xff}))
	require.NoError(t, testTrie.Set([]byte{0xdf}, []byte{0xdf}))
}

func TestIsValid(t *testing.T) {
	mem := NewMemDB()
	defer mem.Close()

	testTrie, err := NewTrie(mem, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	require.NoError(t, testTrie.Set([]byte{0xff}, []byte{0xff}))
	require.NoError(t, testTrie.Set([]byte{0xdf}, []byte{0xdf}))

	p, err := testTrie.GetProof([]byte{0xff})
	require.NoError(t, err)
	ok, err := p.Exists([]byte{0xff})
	require.NoError(t, err)
	require.True(t, ok)

	err = mem.Update(func(b Bucket) error {
		// tamper with one of the leaves, the results should be invalid
		k := p.Leaf.hash(testTrie.nonce)
		leafValBuf := b.Get(k)
		if leafValBuf == nil {
			return xerrors.New("can't find leaf")
		}

		leaf, err := decodeLeafNode(leafValBuf)
		if err != nil {
			return err
		}
		leaf.Value = []byte{0xdf}

		leafValBuf2, err := leaf.encode()
		if err != nil {
			return err
		}

		return b.Put(k, leafValBuf2)
	})
	require.NoError(t, err)
	require.NotNil(t, testTrie.IsValid())
}

func TestQuickCheck(t *testing.T) {
	mem := NewMemDB()
	defer mem.Close()

	testTrie, err := NewTrie(mem, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	f := func(keys [][]byte) bool {
		// Put keys in a map so it's easier to access
		keysMap := make(map[string]struct{})
		for _, k := range keys {
			keysMap[string(k)] = struct{}{}
		}
		// Add a bunch of random keys
		for _, k := range keys {
			if testTrie.Set(k, k) != nil {
				return false
			}
		}
		// Check that they exist
		for _, k := range keys {
			if v, err := testTrie.Get(k); err != nil || v == nil {
				return false
			}
		}
		// Check that everything is in ForEach
		var cnt int
		err := testTrie.ForEach(func(k, v []byte) error {
			cnt++
			if _, ok := keysMap[string(k)]; !ok {
				return xerrors.New("missing key/value pair in foreach")
			}
			return nil
		})
		if err != nil {
			return false
		}
		if cnt != len(keysMap) {
			return false
		}
		// Delete everything
		for _, k := range keys {
			if err := testTrie.Delete(k); err != nil {
				return false
			}
		}
		// We should be left with nothing
		for _, k := range keys {
			if v, err := testTrie.Get(k); err != nil || v != nil {
				return false
			}
		}

		// Check that everything is ok.
		if testTrie.IsValid() != nil {
			return false
		}
		return true
	}
	require.NoError(t, quick.Check(f, nil))
}

type kvPair struct {
	op  OpType
	key []byte
	val []byte
}

func (p kvPair) Op() OpType {
	return p.op
}

func (p kvPair) Key() []byte {
	return p.key
}

func (p kvPair) Val() []byte {
	return p.val
}

func TestBatchQuickCheck(t *testing.T) {
	mem := NewMemDB()
	defer mem.Close()

	testTrie, err := NewTrie(mem, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	f := func(keys [][]byte) bool {
		// Add a bunch of random keys
		var pairs []KVPair
		for _, k := range keys {
			pairs = append(pairs, kvPair{OpSet, k, k})
		}
		if testTrie.Batch(pairs) != nil {
			return false
		}
		// Check that they exist
		for _, k := range keys {
			if v, err := testTrie.Get(k); err != nil || v == nil {
				return false
			}
		}
		// Delete everything
		for _, k := range keys {
			if err := testTrie.Delete(k); err != nil {
				return false
			}
		}
		// We should be left with nothing
		for _, k := range keys {
			if v, err := testTrie.Get(k); err != nil || v != nil {
				return false
			}
		}

		// Check that everything is ok.
		if testTrie.IsValid() != nil {
			return false
		}
		return true
	}
	require.NoError(t, quick.Check(f, nil))
}

func TestCopy(t *testing.T) {
	mem := NewMemDB()
	defer mem.Close()

	disk := newDiskDB(t)
	defer delDiskDB(t, disk)

	testTrie, err := NewTrie(disk, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

	n := 1

	// Create some keys
	for i := 0; i < n; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Set(k, k))
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Make a copy
	trie2, err := NewTrie(mem, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	trie2.noHashKey = true

	err = trie2.DB().Update(func(b Bucket) error {
		return testTrie.CopyTo(b)
	})
	require.NoError(t, err)
	rootKey, err := trie2.getRaw([]byte(entryKey))
	require.NoError(t, err)
	require.NotNil(t, rootKey)

	// Check that everything exists, first on the trie level using Get,
	// then on the storage level using getRaw.
	for i := 0; i < n; i++ {
		k := []byte{byte(i)}
		val, err := trie2.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	err = disk.View(func(b Bucket) error {
		return b.ForEach(func(k, v []byte) error {
			v2, err := trie2.getRaw(k)
			if err != nil {
				return err
			}
			if v2 == nil {
				return xerrors.New("extra node in source trie")
			}
			if !bytes.Equal(v, v2) {
				return xerrors.New("values are not equal")
			}
			return nil
		})
	})
	require.NoError(t, err)
}

func newDiskDB(t *testing.T) DB {
	db, err := bbolt.Open(testDBName, 0600, nil)
	require.NoError(t, err)
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	require.NoError(t, err)
	return NewDiskDB(db, []byte(bucketName))
}

func delDiskDB(t *testing.T, db DB) {
	require.NoError(t, db.Close())
	require.NoError(t, os.Remove(testDBName))
}

func getRootNode(t *testing.T, db DB) interiorNode {
	var root interiorNode
	err := db.View(func(b Bucket) error {
		rootKey := b.Get([]byte(entryKey))
		if rootKey == nil {
			return xerrors.New("no root")
		}
		rootBuf := b.Get(rootKey)
		var err error
		root, err = decodeInteriorNode(rootBuf)
		return err
	})
	require.NoError(t, err)
	return root
}

func genNonce() []byte {
	buf := make([]byte, 32)
	n, err := rand.Read(buf)
	if err != nil {
		return nil
	}
	if n != 32 {
		return nil
	}
	return buf
}

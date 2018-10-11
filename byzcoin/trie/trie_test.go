package trie

import (
	"errors"
	"os"
	"testing"

	bolt "github.com/coreos/bbolt"
	"github.com/stretchr/testify/require"
)

const testDB = "test_trie.db"
const bucketName = "test_trie_bucket"

func Test_NewTrie(t *testing.T) {
	testMemAndDisk(t, testNewTrie)
}

func testNewTrie(t *testing.T, db database) {
	// Initialise a trie.
	trie, err := NewTrie(db, []byte(bucketName))
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)

	// If we iterate the database, we should only have 5 items - the root,
	// the two empty leaves, the entry point and the nonce.
	var cnt int
	db.View(func(tx transaction) error {
		b := tx.Bucket([]byte(bucketName))
		return b.ForEach(func(k, v []byte) error {
			cnt++
			return nil
		})
	})
	require.Equal(t, 5, cnt)

	nonce1 := make([]byte, 32)
	copy(nonce1, trie.nonce)

	// Load the database again and the nonce should still be there.
	trie, err = NewTrie(db, []byte(bucketName))
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)
	require.Equal(t, trie.nonce, nonce1)

	// Check that the root has two children and they exist.
	root := getRootNode(t, db)
	leftBuf, err := trie.getRaw(root.Left)
	require.NoError(t, err)
	left, err := decodeEmptyNode(leftBuf)
	require.NoError(t, err)

	rightBuf, err := trie.getRaw(root.Right)
	require.NoError(t, err)
	right, err := decodeEmptyNode(rightBuf)
	require.NoError(t, err)

	require.Equal(t, root.Left, left.hash(trie.nonce))
	require.Equal(t, root.Right, right.hash(trie.nonce))

	// Load the database again and the root and child should still be there.
	// TODO
}

func Test_AddToEmptyNode(t *testing.T) {
	testMemAndDisk(t, testAddToEmptyNode)
}

func testAddToEmptyNode(t *testing.T, db database) {
	// Initialise a trie.
	trie, err := NewTrie(db, []byte(bucketName))
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)

	// Set two values, which should make the two empty nodes into leaf
	// nodes. 0xff -> MSB is 1, 127 -> MSB is 0
	require.NoError(t, trie.Set([]byte{0xff}, []byte{0xff}))
	require.NoError(t, trie.Set([]byte{127}, []byte{127}))

	val0, err := trie.Get([]byte{0xff})
	require.NoError(t, err)
	require.Equal(t, val0, []byte{0xff})

	val1, err := trie.Get([]byte{127})
	require.NoError(t, err)
	require.Equal(t, val1, []byte{127})

}

func Test_AddToLeafNode(t *testing.T) {
	testMemAndDisk(t, testAddToLeafNode)
}

func testAddToLeafNode(t *testing.T, db database) {
	// Initialise a trie.
	trie, err := NewTrie(db, []byte(bucketName))
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)

	// Set two values, which should make the two empty nodes into leaf
	// nodes. 0xff is 11111111, 0xdf is 11011111. So we need to create new
	// interior nodes.
	require.NoError(t, trie.Set([]byte{0xff}, []byte{0xff}))
	require.NoError(t, trie.Set([]byte{0xdf}, []byte{0xdf}))

	val0, err := trie.Get([]byte{0xff})
	require.NoError(t, err)
	require.Equal(t, val0, []byte{0xff})

	val1, err := trie.Get([]byte{0xdf})
	require.NoError(t, err)
	require.Equal(t, val1, []byte{0xdf})

	// On the other side of the tree, we add nodes on 0x00 and 0x01 to
	// create a even longer list of interior nodes.
	require.NoError(t, trie.Set([]byte{0x00}, []byte{0x00}))
	require.NoError(t, trie.Set([]byte{0x01}, []byte{0x01}))

	val2, err := trie.Get([]byte{0x00})
	require.NoError(t, err)
	require.Equal(t, val2, []byte{0x00})

	val3, err := trie.Get([]byte{0x01})
	require.NoError(t, err)
	require.Equal(t, val3, []byte{0x01})
}

func Test_Overwrite(t *testing.T) {
	testMemAndDisk(t, testOverwrite)
}

func testOverwrite(t *testing.T, db database) {
	// Initialise a trie.
	trie, err := NewTrie(db, []byte(bucketName))
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)

	for i := 0; i < 10; i++ {
		v := []byte{byte(i)}
		require.NoError(t, trie.Set([]byte{0xff}, v))
		val, err := trie.Get([]byte{0xff})
		require.NoError(t, err)
		require.Equal(t, val, v)
	}
}

func Test_Delete(t *testing.T) {
	testMemAndDisk(t, testDelete)
}

func testDelete(t *testing.T, db database) {
	// Initialise a trie.
	trie, err := NewTrie(db, []byte(bucketName))
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)

	// Create some keys
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, trie.Set(k, k))
		val, err := trie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Delete them
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, trie.Delete(k))
	}

	// They should disappear
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		val, err := trie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}

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

func Test_RandomAdd(t *testing.T) {
	// add a bunch of random transactions
	// check that there are no dangling blocks
}

func Test_RandomDelete(t *testing.T) {
	// add a bunch of random nodes
	// delete them in random order
	// at the end the database should be empty
}

func newDiskDB(t *testing.T) database {
	db, err := bolt.Open(testDB, 0600, nil)
	require.NoError(t, err)
	return &diskDB{db}
}

func delDiskDB(t *testing.T, db database) {
	require.NoError(t, db.Close())
	require.NoError(t, os.Remove(testDB))
}

func getRootNode(t *testing.T, db database) interiorNode {
	var root interiorNode
	err := db.View(func(tx transaction) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return errors.New("no bucket name")
		}
		rootKey := b.Get([]byte(entryKey))
		if rootKey == nil {
			return errors.New("no root")
		}
		rootBuf := b.Get(rootKey)
		var err error
		root, err = decodeInteriorNode(rootBuf)
		return err
	})
	require.NoError(t, err)
	return root
}

func testMemAndDisk(t *testing.T, f func(*testing.T, database)) {
	mem := NewMemDB()
	defer mem.Close()
	f(t, mem)

	disk := newDiskDB(t)
	defer delDiskDB(t, disk)
	f(t, disk)
}

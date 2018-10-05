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
	db := newTempDB(t)
	defer deleteDB(t, db)

	// Initialise a trie.
	trie, err := NewTrie(db, []byte(bucketName))
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)

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
	db := newTempDB(t)
	defer deleteDB(t, db)

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
	db := newTempDB(t)
	defer deleteDB(t, db)

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
	db := newTempDB(t)
	defer deleteDB(t, db)

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

func newTempDB(t *testing.T) *bolt.DB {
	db, err := bolt.Open(testDB, 0600, nil)
	require.NoError(t, err)
	return db
}

func deleteDB(t *testing.T, db *bolt.DB) {
	require.NoError(t, db.Close())
	require.NoError(t, os.Remove(testDB))
}

func getRootNode(t *testing.T, db *bolt.DB) interiorNode {
	var root interiorNode
	err := db.View(func(tx *bolt.Tx) error {
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

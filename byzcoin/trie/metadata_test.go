package trie

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	testMemAndDisk(t, testMetadata)
}

func testMetadata(t *testing.T, db DB) {
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	// We cannot set long meta strings
	longKey32 := make([]byte, 32)
	key31 := make([]byte, 31)
	val := []byte("hello")
	require.Error(t, testTrie.SetMetadata(longKey32, val))
	require.Nil(t, testTrie.GetMetadata(longKey32))

	// We cannot set illegal keys
	require.Error(t, testTrie.SetMetadata([]byte(entryKey), val))
	require.Error(t, testTrie.SetMetadata([]byte(nonceKey), val))

	// We can do set/get/del.
	require.NoError(t, testTrie.SetMetadata(key31, val))
	require.Equal(t, val, testTrie.GetMetadata(key31))
	require.NoError(t, testTrie.DeleteMetadata(key31))
	require.Nil(t, testTrie.GetMetadata(key31))

	// Copy includes metadata
	memDB := NewMemDB()
	memTrie, err := NewTrie(memDB, genNonce())
	require.NoError(t, err)
	require.NotNil(t, memTrie.nonce)

	require.NoError(t, testTrie.SetMetadata(key31, val))
	err = memTrie.DB().Update(func(b Bucket) error {
		return testTrie.CopyTo(b)
	})
	require.NoError(t, err)

	require.Equal(t, memTrie.GetRoot(), testTrie.GetRoot())
	require.Equal(t, testTrie.GetMetadata(key31), memTrie.GetMetadata(key31))
	require.Equal(t, val, memTrie.GetMetadata(key31))

}

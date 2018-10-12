package trie

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Ephemeral(t *testing.T) {
	testMemAndDisk(t, testEphemeral)
}

func testEphemeral(t *testing.T, db database) {
	// Initialise a trie.
	testTrie, err := NewTrie(db)
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	// Create some keys
	for i := 90; i < 100; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Set(k, k))
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Create the ephemeral trie
	eTrie := testTrie.MakeEphemeralTrie()

	// Test that we can get values from the source trie
	for i := 90; i < 100; i++ {
		k := []byte{byte(i)}
		val, err := eTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Add new values and test that we can get them.
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Set(k, k))
		val, err := eTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Overwrite values, only go up to 95 (not overwriting everything)
	// because we also want to see what happens when deleting
	// non-overwritten values later.
	for i := 90; i < 95; i++ {
		k := []byte{byte(i)}
		v := []byte{byte(i + 1)}
		require.NoError(t, eTrie.Set(k, v))
		val, err := eTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, v)
	}

	// Delete values from the ephemeral trie
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Delete(k))
		val, err := eTrie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}

	// Delete values from the source trie
	for i := 90; i < 100; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Delete(k))
		val, err := eTrie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}
}

func Test_EphemeralCommit(t *testing.T) {
	testMemAndDisk(t, testEphemeralCommit)
}

func testEphemeralCommit(t *testing.T, db database) {
	// Initialise a trie.
	testTrie, err := NewTrie(db)
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	eTrie := testTrie.MakeEphemeralTrie()

	// Make set/delete transactions and then commit, make sure they exist
	// in the source Trie.
	// Test that we can get values from the source trie
	for i := 100; i < 200; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Set(k, k))
	}
	for i := 100; i < 150; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Delete(k))
	}

	require.NoError(t, eTrie.Commit())

	for i := 100; i < 150; i++ {
		// missing
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Nil(t, val)
	}
	for i := 150; i < 200; i++ {
		// exists
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Make more transactions and commit, check that old ones stay the same
	// and new ones exist.
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Set(k, k))
	}
	for i := 100; i < 150; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Set(k, k))
	}

	require.NoError(t, eTrie.Commit())

	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}
	for i := 100; i < 200; i++ {
		k := []byte{byte(i)}
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}
}

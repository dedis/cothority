package trie

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProof(t *testing.T) {
	testMemAndDisk(t, testProof)
}

func testProof(t *testing.T, db database) {
	// Initialise a trie.
	trie, err := NewTrie(db)
	require.NoError(t, err)
	require.NotNil(t, trie.nonce)

	// Create some keys
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, trie.Set(k, k))
		val, err := trie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Create and verify proof
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		p, err := trie.GetProof(k)
		require.NoError(t, err)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// Check thet proofs don't exist in other keys
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		p, err := trie.GetProof(k)
		require.NoError(t, err)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Delete the keys and the proof should not exist
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, trie.Delete(k))

		p, err := trie.GetProof(k)
		require.NoError(t, err)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)
	}
}

func TestProof_Randomised(t *testing.T) {
	// TODO have two sets of keys
	// insert the keys from one set
	// randomly get proofs from both sets
}

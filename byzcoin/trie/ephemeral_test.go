package trie

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEphemeral(t *testing.T) {
	testMemAndDisk(t, testEphemeral)
}

func testEphemeral(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
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

func TestEphemeralCommit(t *testing.T) {
	testMemAndDisk(t, testEphemeralCommit)
}

func testEphemeralCommit(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)
	testTrie.noHashKey = true

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

func TestEphemeralGetRoot(t *testing.T) {
	disk := newDiskDB(t)
	defer delDiskDB(t, disk)

	testTrie, err := NewTrie(disk, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	eTrie := testTrie.MakeEphemeralTrie()

	// We should start with the same root
	initialRoot := testTrie.GetRoot()
	require.NotNil(t, initialRoot)
	require.Equal(t, initialRoot, eTrie.GetRoot())

	// The root of the ephemeral trie should match the real trie after
	// making operations.
	for i := 100; i < 200; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Set(k, k))
	}
	for i := 100; i < 150; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Delete(k))
	}
	eRoot := eTrie.GetRoot()

	// The initial root shouldn't change.
	require.Equal(t, initialRoot, testTrie.GetRoot())

	// Commit the ephemeral trie, then the source root should match the
	// root on the previously computed ephemeral trie.
	require.NoError(t, eTrie.Commit())
	require.Equal(t, eRoot, testTrie.GetRoot())
	require.Equal(t, eRoot, eTrie.GetRoot())
}

func TestEphemeralGetProof(t *testing.T) {
	disk := newDiskDB(t)
	defer delDiskDB(t, disk)

	testTrie, err := NewTrie(disk, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	eTrie := testTrie.MakeEphemeralTrie()

	// Make some ephemeral operations and check for proofs.
	var ephExistProof []*Proof
	var ephAbsenceProof []*Proof
	for i := 0; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Set(k, k))
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, eTrie.Delete(k))
	}

	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		p, err := eTrie.GetProof(k)
		require.NoError(t, err)
		require.True(t, p.Match(k))
		ephExistProof = append(ephExistProof, p)
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		p, err := eTrie.GetProof(k)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)
		ephAbsenceProof = append(ephAbsenceProof, p)
	}

	// Commit the operations and make the same proofs on the source trie,
	// make sure they're the same.
	require.NoError(t, eTrie.Commit())
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		p, err := testTrie.GetProof(k)
		require.NoError(t, err)
		require.True(t, p.Match(k))

		// The root and the leaf are the same, so the proof must be the
		// same due to properties of hash functions.
		require.Equal(t, ephExistProof[i].GetRoot(), p.GetRoot())
		require.Equal(t, ephExistProof[i].Leaf.hash(eTrie.source.nonce), p.Leaf.hash(testTrie.nonce))
	}
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		p, err := testTrie.GetProof(k)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)

		// The root and the leaf are the same, so the proof must be the
		// same due to properties of hash functions.
		require.Equal(t, ephAbsenceProof[i-10].GetRoot(), p.GetRoot())
		require.Equal(t, ephAbsenceProof[i-10].Empty.hash(eTrie.source.nonce), p.Empty.hash(testTrie.nonce))
	}
}

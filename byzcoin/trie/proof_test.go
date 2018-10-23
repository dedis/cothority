package trie

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"
)

func TestProof(t *testing.T) {
	testMemAndDisk(t, testProof)
}

func testProof(t *testing.T, db DB) {
	// Initialise a trie.
	testTrie, err := NewTrie(db, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	// Create some keys
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Set(k, k))
		val, err := testTrie.Get(k)
		require.NoError(t, err)
		require.Equal(t, val, k)
	}

	// Create and verify proof
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		p, err := testTrie.GetProof(k)
		require.NoError(t, err)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// Check thet proofs don't exist in other keys
	for i := 0; i < 10; i++ {
		k := []byte{byte(i)}
		p, err := testTrie.GetProof(k)
		require.NoError(t, err)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)

		// Check that the roots are the same.
		require.Equal(t, testTrie.GetRoot(), p.GetRoot())
	}

	// Delete the keys and the proof should not exist
	for i := 10; i < 20; i++ {
		k := []byte{byte(i)}
		require.NoError(t, testTrie.Delete(k))

		p, err := testTrie.GetProof(k)
		require.NoError(t, err)
		ok, err := p.Exists(k)
		require.NoError(t, err)
		require.False(t, ok)

		// Check that the roots are the same.
		require.Equal(t, testTrie.GetRoot(), p.GetRoot())
	}

}

type disjointSet struct {
	A [][]byte
	B [][]byte
}

func (disjointSet) Generate(rand *rand.Rand, size int) reflect.Value {
	res := disjointSet{}
	start := rand.Uint32()
	for i := 0; i < size; i++ {
		iA := start + uint32(i)
		bufA := make([]byte, 4)
		binary.LittleEndian.PutUint32(bufA, iA)
		elemA := sha256.Sum256(bufA)
		res.A = append(res.A, elemA[:])
	}
	for i := size; i < size*2; i++ {
		iB := start + uint32(i)
		bufB := make([]byte, 4)
		binary.LittleEndian.PutUint32(bufB, iB)
		elemB := sha256.Sum256(bufB)
		res.B = append(res.B, elemB[:])
	}
	return reflect.ValueOf(res)
}

func TestProofQuickCheck(t *testing.T) {
	mem := NewMemDB()
	defer mem.Close()

	testTrie, err := NewTrie(mem, genNonce())
	require.NoError(t, err)
	require.NotNil(t, testTrie.nonce)

	f := func(s disjointSet) bool {
		// Add a bunch of random keys
		for _, k := range s.A {
			if testTrie.Set(k, k) != nil {
				return false
			}
		}
		// Check that we can get proof
		for _, k := range s.A {
			p, err := testTrie.GetProof(k)
			if err != nil {
				return false
			}
			ok, err := p.Exists(k)
			if err != nil {
				return false
			}
			if !ok {
				return false
			}
			// Check that key/values exist and are correct.
			k2, v2 := p.KeyValue()
			if !bytes.Equal(k2, k) {
				return false
			}
			if !bytes.Equal(v2, k) {
				return false
			}
			if !bytes.Equal(p.Get(k), k) {
				return false
			}
		}
		// Check that there are no proofs for the other set.
		for _, k := range s.B {
			p, err := testTrie.GetProof(k)
			if err != nil {
				return false
			}
			ok, err := p.Exists(k)
			if err != nil {
				return false
			}
			if ok {
				return false
			}
			// Check that key/values do not exist
			k2, _ := p.KeyValue()
			if bytes.Equal(k2, k) {
				return false
			}
			if p.Get(k) != nil {
				return false
			}
		}
		return true
	}
	require.NoError(t, quick.Check(f, nil))
}

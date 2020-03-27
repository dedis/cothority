package patriciatrie

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	byzcointrie "go.dedis.ch/cothority/v3/byzcoin/trie"
	"golang.org/x/xerrors"
)

func Test_TriePutEmptyRoot(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	expectedHash, _ := hex.DecodeString("3f501138c2daf7feda1ebb93c79427ea23635b8e1b20272d40657a84da189313")
	require.Equal(t, trie.RootHash(), expectedHash)
}

func Test_TriePut(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	expectedHash, _ := hex.DecodeString("3f501138c2daf7feda1ebb93c79427ea23635b8e1b20272d40657a84da189313")
	require.Equal(t, trie.RootHash(), expectedHash)

	err = trie.Put([]byte("foobar"), []byte("bar"))
	require.NoError(t, err)

	expectedHash, _ = hex.DecodeString("df1ea1f9390dacc30a1bd1affa848f9cf39a0ff49e0b7dd2280f31116f70dd27")
	require.Equal(t, trie.RootHash(), expectedHash)
}

func Test_TriePutDifferentPrefix(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	expectedHash, _ := hex.DecodeString("3f501138c2daf7feda1ebb93c79427ea23635b8e1b20272d40657a84da189313")
	require.Equal(t, trie.RootHash(), expectedHash)

	err = trie.Put([]byte("anotherkey"), []byte("anotherval"))
	require.NoError(t, err)

	expectedHash, _ = hex.DecodeString("64f7e9acc2eedc98b6b6ec8958c874c88076acdf2de951a6dc75535a2a96de6f")
	require.Equal(t, trie.RootHash(), expectedHash)
}

func Test_TriePutUpdateKey(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	err = trie.Put([]byte("foobar"), []byte("bar"))
	require.NoError(t, err)

	err = trie.Put([]byte("foobar"), []byte("baz"))
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("itworks"))
	require.NoError(t, err)

	expectedHash, _ := hex.DecodeString("b25d905c498d8a3ea5b4f5afcfd043fc85cbd0003b9bc744dbcd0eb1771611b1")
	require.Equal(t, trie.RootHash(), expectedHash)
}

func Test_TrieGetEmpty(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	_, err = trie.Get([]byte("foo"))
	require.Equal(t, keyNotFoundError, err)
}

func Test_TrieGetNonExistingKey(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	_, err = trie.Get([]byte("fo"))
	require.Equal(t, keyNotFoundError, err)

	_, err = trie.Get([]byte("foobar"))
	require.Equal(t, keyNotFoundError, err)

	val, err := trie.Get([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), val)
}

func Test_TrieGetKeysCommonPrefix(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	err = trie.Put([]byte("foobar"), []byte("baz"))
	require.NoError(t, err)

	err = trie.Put([]byte("foobarrrr"), []byte("itworks"))
	require.NoError(t, err)

	val, err := trie.Get([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), val)

	val, err = trie.Get([]byte("foobar"))
	require.NoError(t, err)
	require.Equal(t, []byte("baz"), val)

	val, err = trie.Get([]byte("foobarrrr"))
	require.NoError(t, err)
	require.Equal(t, []byte("itworks"), val)

	expectedHash, _ := hex.DecodeString("b575519055fd25b4fff3fec4df1a79c4d8d770ec211e14dcababd001151959c0")
	require.Equal(t, expectedHash, trie.RootHash())
}

func Test_TrieGetKeysCommonPrefixReverseOrder(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foobarrrr"), []byte("itworks"))
	require.NoError(t, err)

	err = trie.Put([]byte("foobar"), []byte("baz"))
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	val, err := trie.Get([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), val)

	val, err = trie.Get([]byte("foobar"))
	require.NoError(t, err)
	require.Equal(t, []byte("baz"), val)

	val, err = trie.Get([]byte("foobarrrr"))
	require.NoError(t, err)
	require.Equal(t, []byte("itworks"), val)

	expectedHash, _ := hex.DecodeString("b575519055fd25b4fff3fec4df1a79c4d8d770ec211e14dcababd001151959c0")
	require.Equal(t, expectedHash, trie.RootHash())
}

func Test_TrieGetKeysCommonPrefixAnotherOrder(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foobarrrr"), []byte("itworks"))
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	err = trie.Put([]byte("foobar"), []byte("baz"))
	require.NoError(t, err)

	val, err := trie.Get([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), val)

	val, err = trie.Get([]byte("foobar"))
	require.NoError(t, err)
	require.Equal(t, []byte("baz"), val)

	val, err = trie.Get([]byte("foobarrrr"))
	require.NoError(t, err)
	require.Equal(t, []byte("itworks"), val)

	expectedHash, _ := hex.DecodeString("b575519055fd25b4fff3fec4df1a79c4d8d770ec211e14dcababd001151959c0")
	require.Equal(t, expectedHash, trie.RootHash())
}

func Test_TrieCommit(t *testing.T) {
	trie, err := NewPatriciaTrie(byzcointrie.NewMemDB())
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("bar"))
	require.NoError(t, err)

	trie.Put([]byte("foobar"), []byte("bar"))
	require.NoError(t, err)

	err = trie.Commit()
	require.NoError(t, err)

	err = trie.db.View(func(b byzcointrie.Bucket) error {
		val := b.Get(trie.rootKey)
		if len(val) == 0 {
			return xerrors.New("root not found")
		}
		return nil
	})
	require.NoError(t, err)
}

func Test_TrieLoad(t *testing.T) {
	memDb := byzcointrie.NewMemDB()
	trie, err := NewPatriciaTrie(memDb)
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("itworks"))
	require.NoError(t, err)

	trie.Put([]byte("foobar"), []byte("bar"))
	require.NoError(t, err)

	err = trie.Commit()
	require.NoError(t, err)

	trie2, err := LoadPatriciaTrie(memDb)
	require.NoError(t, err)

	val, err := trie2.Get([]byte("foobar"))
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), val)

	val, err = trie2.Get([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, []byte("itworks"), val)
}

func Test_TrieGetAtRoot(t *testing.T) {
	memDb := byzcointrie.NewMemDB()
	trie, err := NewPatriciaTrie(memDb)
	require.NoError(t, err)

	err = trie.Put([]byte("foo"), []byte("itworks"))
	require.NoError(t, err)

	trie.Put([]byte("foobar"), []byte("bar"))
	require.NoError(t, err)

	err = trie.Commit()
	rootHash := trie.RootHash()

	err = trie.Put([]byte("foobar"), []byte("newval"))
	require.NoError(t, err)
	err = trie.Commit()
	require.NoError(t, err)
	data, err := trie.Get([]byte("foobar"))
	require.NoError(t, err)
	require.Equal(t, []byte("newval"), data)

	data, err = trie.GetAtRoot(rootHash, []byte("foobar"))
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), data)
}

package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	bolt "github.com/coreos/bbolt"
	"github.com/stretchr/testify/require"
)

var testName = []byte("coll1")

func TestCollectionDBStrange(t *testing.T) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	cdb := newCollectionDB(db, testName)
	key := []byte("first")
	value := []byte("value")
	kind := []byte("mykind")
	err = cdb.Store(&Transaction{
		Key:   key,
		Value: value,
		Kind:  kind,
	})
	require.Nil(t, err)
	v, k, err := cdb.GetValueKind([]byte("first"))
	require.Nil(t, err)
	require.Equal(t, value, v)
	require.Equal(t, kind, k)
}

func TestCollectionDB(t *testing.T) {
	kvPairs := 16

	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	cdb := newCollectionDB(db, testName)
	pairs := map[string]string{}
	mykind := []byte("mykind")
	for i := 0; i < kvPairs; i++ {
		pairs[fmt.Sprintf("Key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Store all key/value pairs
	for k, v := range pairs {
		tr := &Transaction{
			Key:   []byte(k),
			Value: []byte(v),
			Kind:  mykind,
		}
		require.Nil(t, cdb.Store(tr))
	}

	// Verify it's all there
	for k, v := range pairs {
		stored, kind, err := cdb.GetValueKind([]byte(k))
		require.Nil(t, err)
		require.Equal(t, v, string(stored))
		require.Equal(t, mykind, kind)
	}

	// Get a new db handler
	cdb2 := newCollectionDB(db, testName)

	// Verify it's all there
	for k, v := range pairs {
		stored, _, err := cdb2.GetValueKind([]byte(k))
		require.Nil(t, err)
		require.Equal(t, v, string(stored))
	}
}

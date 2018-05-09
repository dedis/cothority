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
	err = cdb.Store(&StateChange{
		Action: Create,
		Key:    key,
		Value:  value,
		Kind:   kind,
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
		sc := &StateChange{
			Action: Create,
			Key:    []byte(k),
			Value:  []byte(v),
			Kind:   mykind,
		}
		require.Nil(t, cdb.Store(sc))
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

// TODO: Test good case, bad add case, bad remove case
func TestCollectionDBtryHash(t *testing.T) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	cdb := newCollectionDB(db, testName)
	ts := []OmniledgerTransaction{{
		StateChanges: []StateChange{{
			Action: Create,
			Key:    []byte("key1"),
			Kind:   []byte("kind1"),
			Value:  []byte("value1"),
		},
			{
				Action: Create,
				Key:    []byte("key2"),
				Kind:   []byte("kind2"),
				Value:  []byte("value2"),
			},
		},
	}}
	mrTrial, err := cdb.tryHash(ts)
	require.Nil(t, err)
	_, _, err = cdb.GetValueKind([]byte("key1"))
	require.EqualError(t, err, "no match found")
	_, _, err = cdb.GetValueKind([]byte("key2"))
	require.EqualError(t, err, "no match found")
	cdb.Store(&ts[0].StateChanges[0])
	cdb.Store(&ts[0].StateChanges[1])
	mrReal := cdb.RootHash()
	require.Equal(t, mrTrial, mrReal)
}

func TestSortTransactions(t *testing.T) {
	ts1 := []ClientTransaction{{
		Instructions: []Instruction{{
			DarcID:  []byte("key1"),
			Nonce:   []byte("nonce1"),
			Command: "Create",
			Kind:    "kind1",
			Data:    []byte("value1"),
		}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key2"),
				Nonce:   []byte("nonce2"),
				Command: "Create",
				Kind:    "kind2",
				Data:    []byte("value2"),
			}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key2"),
				Nonce:   []byte("nonce2"),
				Command: "Create",
				Kind:    "kind2",
				Data:    []byte("value2"),
			}}},
	}
	ts2 := []ClientTransaction{{
		Instructions: []Instruction{{
			DarcID:  []byte("key2"),
			Nonce:   []byte("nonce2"),
			Command: "Create",
			Kind:    "kind2",
			Data:    []byte("value2"),
		}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key1"),
				Nonce:   []byte("nonce1"),
				Command: "Create",
				Kind:    "kind1",
				Data:    []byte("value1"),
			}}},
		{
			Instructions: []Instruction{{
				DarcID:  []byte("key2"),
				Nonce:   []byte("nonce2"),
				Command: "Create",
				Kind:    "kind2",
				Data:    []byte("value2"),
			}}},
	}
	err := sortTransactions(ts1)
	require.Nil(t, err)
	err = sortTransactions(ts2)
	require.Nil(t, err)
	for i := range ts1 {
		require.Equal(t, ts1[i], ts2[i])
	}
}

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
	contract := []byte("mycontract")
	err = cdb.Store(&StateChange{
		StateAction: Create,
		ObjectID:    key,
		Value:       value,
		ContractID:  contract,
	})
	require.Nil(t, err)
	v, c, err := cdb.GetValueContract([]byte("first"))
	require.Nil(t, err)
	require.Equal(t, value, v)
	require.Equal(t, contract, c)
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
	myContract := []byte("myContract")
	for i := 0; i < kvPairs; i++ {
		pairs[fmt.Sprintf("Key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Store all key/value pairs
	for k, v := range pairs {
		sc := &StateChange{
			StateAction: Create,
			ObjectID:    []byte(k),
			Value:       []byte(v),
			ContractID:  myContract,
		}
		require.Nil(t, cdb.Store(sc))
	}

	// Verify it's all there
	for c, v := range pairs {
		stored, contract, err := cdb.GetValueContract([]byte(c))
		require.Nil(t, err)
		require.Equal(t, v, string(stored))
		require.Equal(t, myContract, contract)
	}

	// Get a new db handler
	cdb2 := newCollectionDB(db, testName)

	// Verify it's all there
	for c, v := range pairs {
		stored, _, err := cdb2.GetValueContract([]byte(c))
		require.Nil(t, err)
		require.Equal(t, v, string(stored))
	}

	// Update
	for k, v := range pairs {
		pairs[k] = v + "-2"
	}
	for k, v := range pairs {
		sc := &StateChange{
			StateAction: Update,
			ObjectID:    []byte(k),
			Value:       []byte(v),
			ContractID:  myContract,
		}
		require.Nil(t, cdb2.Store(sc), k)
	}
	for k, v := range pairs {
		stored, contract, err := cdb2.GetValueContract([]byte(k))
		require.Nil(t, err)
		require.Equal(t, v, string(stored))
		require.Equal(t, myContract, contract)
	}

	// Delete
	for c := range pairs {
		sc := &StateChange{
			StateAction: Remove,
			ObjectID:    []byte(c),
			ContractID:  myContract,
		}
		require.Nil(t, cdb2.Store(sc))
	}
	for c := range pairs {
		_, _, err := cdb2.GetValueContract([]byte(c))
		require.NotNil(t, err, c)
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
	scs := []StateChange{{
		StateAction: Create,
		ObjectID:    []byte("key1"),
		ContractID:  []byte("kind1"),
		Value:       []byte("value1"),
	},
		{
			StateAction: Create,
			ObjectID:    []byte("key2"),
			ContractID:  []byte("kind2"),
			Value:       []byte("value2"),
		},
	}
	mrTrial, err := cdb.tryHash(scs)
	require.Nil(t, err)
	_, _, err = cdb.GetValueContract([]byte("key1"))
	require.EqualError(t, err, "no match found")
	_, _, err = cdb.GetValueContract([]byte("key2"))
	require.EqualError(t, err, "no match found")
	cdb.Store(&scs[0])
	cdb.Store(&scs[1])
	mrReal := cdb.RootHash()
	require.Equal(t, mrTrial, mrReal)
}

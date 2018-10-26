package byzcoin

import (
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	bolt "github.com/coreos/bbolt"
	"github.com/stretchr/testify/require"
)

func generateStateChanges(id []byte) StateChanges {
	scs := StateChanges{}
	for i := 0; i < 10; i++ {
		scs = append(scs, StateChange{
			InstanceID: id,
			Value:      []byte{byte(i)},
			Version:    int(i),
		})
	}

	return scs
}

func TestStateChangeStorage(t *testing.T) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	scs := stateChangeStorage{
		db: db,
	}

	ss := append(generateStateChanges([]byte{1}), generateStateChanges([]byte{2})...)
	perm := rand.Perm(len(ss))
	for i, j := range perm {
		ss[i], ss[j] = ss[j], ss[i]
	}

	err = scs.append(ss)
	require.Nil(t, err)

	entries, err := scs.getAll([]byte{1})
	require.Nil(t, err)
	require.Equal(t, 10, len(entries))

	for i := 0; i < 10; i++ {
		require.Equal(t, i, entries[i].StateChange.Version)
		e, ok, err := scs.getByVersion([]byte{1}, i)

		require.Nil(t, err)
		require.True(t, ok)
		require.Equal(t, e.StateChange.Value, entries[i].StateChange.Value)
	}

	_, ok, err := scs.getByVersion([]byte{3}, 0)
	require.False(t, ok)
	require.Nil(t, err)

	db.Close()
}

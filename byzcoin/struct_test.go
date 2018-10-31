package byzcoin

import (
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/skipchain"
	"github.com/stretchr/testify/require"
)

func generateStateChanges(id []byte) StateChanges {
	scs := StateChanges{}
	for i := 0; i < 10; i++ {
		scs = append(scs, StateChange{
			InstanceID: id,
			Value:      []byte{byte(i)},
			Version:    uint64(i),
		})
	}

	return scs
}

func TestStateChangeStorage_SimpleCase(t *testing.T) {
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

	err = scs.append(ss, skipchain.NewSkipBlock())
	require.Nil(t, err)

	entries, err := scs.getAll([]byte{1})
	require.Nil(t, err)
	require.Equal(t, 10, len(entries))

	for i := 0; i < 10; i++ {
		require.Equal(t, uint64(i), entries[i].StateChange.Version)
		e, ok, err := scs.getByVersion([]byte{1}, uint64(i))

		require.Nil(t, err)
		require.True(t, ok)
		require.Equal(t, e.StateChange.Value, entries[i].StateChange.Value)
	}

	_, ok, err := scs.getByVersion([]byte{3}, 0)
	require.False(t, ok)
	require.Nil(t, err)

	db.Close()
}

func TestStateChangeStorage_MaxSize(t *testing.T) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	scs := stateChangeStorage{
		db:      db,
		maxSize: 1024 * 10,
	}

	for i := 0; i < 100; i++ {
		scs.append(StateChanges{StateChange{
			InstanceID: []byte{1, 2, 3},
			Version:    uint64(i),
			Value:      make([]byte, 1000),
		}}, skipchain.NewSkipBlock())
	}

	entries, err := scs.getAll([]byte{1, 2, 3})
	require.Nil(t, err)
	require.Equal(t, 10, len(entries))
	require.Equal(t, uint64(99), entries[9].StateChange.Version)
}

func TestStateChangeStorage_MaxNbrBlock(t *testing.T) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	scs := stateChangeStorage{
		db:          db,
		maxNbrBlock: 5,
	}

	sb := skipchain.NewSkipBlock()
	for i := 0; i < 100; i++ {
		sb.Index = i

		scs.append(StateChanges{StateChange{
			InstanceID: []byte{byte(i % 5)},
			Version:    uint64(i),
			Value:      []byte{},
		}}, sb)
	}

	entries, err := scs.getAll([]byte{0})
	require.Nil(t, err)
	require.Equal(t, 5, len(entries))
	require.Equal(t, 95, entries[4].BlockIndex)
}

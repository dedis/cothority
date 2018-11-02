package byzcoin

import (
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/require"
)

func TestStateChangeStorage_SimpleCase(t *testing.T) {
	scs, name := generateDB(t)
	defer os.Remove(name)

	ss := append(generateStateChanges(), generateStateChanges()...)
	perm := rand.Perm(len(ss))
	for i, j := range perm {
		ss[i], ss[j] = ss[j], ss[i]
	}

	err := scs.append(ss, skipchain.NewSkipBlock())
	require.Nil(t, err)

	entries, err := scs.getAll(ss[0].InstanceID)
	require.Nil(t, err)
	require.Equal(t, 10, len(entries))

	for i := 0; i < 10; i++ {
		require.Equal(t, uint64(i), entries[i].StateChange.Version)
		e, ok, err := scs.getByVersion(ss[0].InstanceID, uint64(i))

		require.Nil(t, err)
		require.True(t, ok)
		require.Equal(t, e.StateChange.Value, entries[i].StateChange.Value)
	}

	fakeID := genID().Slice()
	_, ok, err := scs.getByVersion(fakeID, 0)
	require.False(t, ok)
	require.Nil(t, err)

	sce, ok, err := scs.getLast(ss[0].InstanceID)
	require.Nil(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(9), sce.StateChange.Version)
}

func TestStateChangeStorage_MaxSize(t *testing.T) {
	scs, name := generateDB(t)
	defer os.Remove(name)

	n := 20
	size := 10
	iid1 := genID().Slice()
	iid2 := genID().Slice()

	sc := StateChange{
		InstanceID: iid1,
		Version:    uint64(0),
		Value:      make([]byte, 1000),
	}
	buf, err := protobuf.Encode(&sc)
	require.Nil(t, err)

	scs.maxSize = len(buf) * size

	for i := 0; i < n; i++ {
		sc.Version = uint64(i)
		scs.append(StateChanges{sc}, skipchain.NewSkipBlock())
	}

	sc.InstanceID = iid2

	for i := 0; i < n; i++ {
		sc.Version = uint64(i)
		scs.append(StateChanges{sc}, skipchain.NewSkipBlock())
	}

	entries, err := scs.getAll(iid2)
	require.Nil(t, err)
	require.Equal(t, size, len(entries))

	for i := 0; i < size; i++ {
		require.Equal(t, uint64(n-size+i), entries[i].StateChange.Version)
	}

	entries, err = scs.getAll(iid1)
	require.Nil(t, err)
	require.Equal(t, 0, len(entries))
}

func TestStateChangeStorage_MaxNbrBlock(t *testing.T) {
	scs, name := generateDB(t)
	scs.maxNbrBlock = 2
	defer os.Remove(name)

	n := 50
	k := 5

	iids := make([][]byte, k)
	for i := range iids {
		iids[i] = genID().Slice()
	}

	sb := skipchain.NewSkipBlock()
	for i := 0; i < n; i++ {
		sb.Index = i

		scs.append(StateChanges{StateChange{
			InstanceID: iids[i%k],
			Version:    uint64(i),
			Value:      []byte{},
		}}, sb)
	}

	entries, err := scs.getAll(iids[k-1])
	require.Nil(t, err)
	require.Equal(t, 2, len(entries))
	require.Equal(t, n-1, entries[1].BlockIndex)
}

func generateStateChanges() StateChanges {
	id := genID().Slice()

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

func generateDB(t *testing.T) (*stateChangeStorage, string) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	scs := stateChangeStorage{db: db, bucket: []byte("scstest")}
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(scs.bucket)
		return err
	})

	return &scs, tmpDB.Name()
}

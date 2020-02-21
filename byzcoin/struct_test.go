package byzcoin

import (
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/protobuf"
	bbolt "go.etcd.io/bbolt"
)

func createBlock() *skipchain.SkipBlock {
	sb := skipchain.NewSkipBlock()
	nonce := GenNonce()
	sb.Data = nonce[:]
	sb.Hash = sb.CalculateHash()
	sb.GenesisID = sb.Hash

	return sb
}

// Checks that the size of the storage is correctly restored
// after reading the DB and that the indices are correct
func TestStateChangeStorage_Init(t *testing.T) {
	scs, name := generateDB(t)
	defer os.Remove(name)

	n := 3
	k := 10

	size := 0
	sbs := make([]*skipchain.SkipBlock, n)
	for i := range sbs {
		sbs[i] = skipchain.NewSkipBlock()
		sbs[i].Data = []byte{byte(i)}
		sbs[i].Hash = sbs[i].CalculateHash()
	}

	// Put random values and increment the block indices to
	// check the size and indices initialisation
	err := scs.db.Update(func(tx *bbolt.Tx) error {
		for i := 0; i < n; i++ {
			b := tx.Bucket(scs.bucket)

			scb, err := b.CreateBucketIfNotExists(sbs[i].Hash)
			if err != nil {
				return err
			}

			for j := 0; j < k; j++ {
				key := make([]byte, prefixLength+versionLength+8)
				key[len(key)-1] = byte(j)

				d := GenNonce()
				scb.Put(key, d[:])
				size += len(d)
			}
		}

		return nil
	})
	require.NoError(t, err)

	err = scs.calculateSize()
	require.NoError(t, err)
	require.Equal(t, size, scs.size)
}

// Checks basic usage of the state change storage
func TestStateChangeStorage_SimpleCase(t *testing.T) {
	scs, name := generateDB(t)
	defer os.Remove(name)

	ss := append(generateStateChanges(10), generateStateChanges(10)...)
	perm := rand.Perm(len(ss))
	for i, j := range perm {
		ss[i], ss[j] = ss[j], ss[i]
	}

	sb := createBlock()
	err := scs.append(ss, sb)
	require.NoError(t, err)

	// check the size remains the same with already seen state changes
	size := scs.size
	err = scs.append(ss, sb)
	require.NoError(t, err)
	require.Equal(t, size, scs.size)

	entries, err := scs.getAll(ss[0].InstanceID, sb.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, 10, len(entries))

	for i := 0; i < 10; i++ {
		require.Equal(t, uint64(i), entries[i].StateChange.Version)
		e, ok, err := scs.getByVersion(ss[0].InstanceID, uint64(i), sb.SkipChainID())

		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, e.StateChange.Value, entries[i].StateChange.Value)
	}

	fakeID := genID().Slice()
	_, ok, err := scs.getByVersion(fakeID, 0, sb.SkipChainID())
	require.False(t, ok)
	require.NoError(t, err)

	sce, ok, err := scs.getLast(ss[0].InstanceID, sb.SkipChainID())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(9), sce.StateChange.Version)
}

// Checks that GetByBlock returns the actual list of state
// changes for this block
func TestStateChangeStorage_GetByBlock(t *testing.T) {
	store, name := generateDB(t)
	defer os.Remove(name)

	n := 5
	k := 3

	sbs := make([]*skipchain.SkipBlock, n)
	for i := range sbs {
		sbs[i] = createBlock()
		sbs[i].Index = i
		sbs[i].GenesisID = sbs[0].Hash

		for j := 0; j < k; j++ {
			sc := StateChange{
				InstanceID: genID().Slice(),
				Version:    uint64(i*k + j),
				Value:      []byte{},
			}
			err := store.append(StateChanges{sc}, sbs[i])
			require.NoError(t, err)
		}
	}

	sce, err := store.getByBlock(sbs[n-1].SkipChainID(), 0)
	require.NoError(t, err)
	require.Equal(t, k, len(sce))
}

// Checks the independance of the skipchains for the state changes
func TestStateChangeStorage_MultiSkipChain(t *testing.T) {
	store, name := generateDB(t)
	defer os.Remove(name)

	n := 3
	k := 5

	iid := make([]byte, prefixLength)
	chains := make([]*skipchain.SkipBlock, n)
	for i := range chains {
		chains[i] = createBlock()

		for j := 0; j < k; j++ {
			chains[i].Index = j
			sc := StateChange{
				InstanceID: iid,
				Version:    uint64(j),
				Value:      []byte{},
			}
			err := store.append(StateChanges{sc}, chains[i])
			require.NoError(t, err)
		}
	}

	for _, chain := range chains {
		sce, err := store.getAll(iid, chain.SkipChainID())
		require.NoError(t, err)
		require.Equal(t, k, len(sce))

		sce, err = store.getByBlock(chain.SkipChainID(), k-1)
		require.NoError(t, err)
		require.Equal(t, 1, len(sce))

		e, ok, err := store.getLast(iid, chain.SkipChainID())
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, uint64(k-1), e.StateChange.Version)

		e, ok, err = store.getByVersion(iid, uint64(1), chain.SkipChainID())
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, uint64(1), e.StateChange.Version)
	}
}

// Checks the max size parameter is correctly taken in account
// and that the storage keeps the size below the threshold
func TestStateChangeStorage_MaxSize(t *testing.T) {
	store, name := generateDB(t)
	defer os.Remove(name)

	n := 20
	size := 2
	iid1 := genID().Slice()
	iid2 := genID().Slice()
	// check over 2 skipchains as we clean independently from the skipchain
	sb1 := createBlock()
	sb2 := createBlock()

	sc := StateChange{
		InstanceID: iid1,
		Version:    uint64(0),
		Value:      make([]byte, 200),
	}
	buf, err := protobuf.Encode(&sc)
	require.NoError(t, err)

	store.maxSize = len(buf) * size

	for i := 0; i < n; i++ {
		sc.Version = uint64(i)
		sb1.Index = i
		err = store.append(StateChanges{sc}, sb1)
		require.NoError(t, err)
	}

	sc.InstanceID = iid2

	for i := 0; i < n; i++ {
		sc.Version = uint64(i)
		sb2.Index = i
		err = store.append(StateChanges{sc}, sb2)
		require.NoError(t, err)
	}

	entries, err := store.getAll(iid2, sb2.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, size, len(entries))

	for i := 0; i < size; i++ {
		require.Equal(t, uint64(n-size+i), entries[i].StateChange.Version)
	}

	entries, err = store.getAll(iid1, sb1.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, 0, len(entries))
}

// Checks that the parameter of the maximum number of blocks is taken
// in account and that older blocks are cleaned
func TestStateChangeStorage_MaxNbrBlock(t *testing.T) {
	store, name := generateDB(t)
	store.maxNbrBlock = 2
	defer os.Remove(name)

	k := 3
	l := 4
	n := l * 12

	iids := make([][]byte, k)
	for i := range iids {
		iids[i] = genID().Slice()
	}

	sb := createBlock()
	for i := 0; i < n; i++ {
		sb.Index = i / l
		var scs StateChanges

		for j := 0; j < k; j++ {
			scs = append(scs, StateChange{
				InstanceID: iids[j],
				Version:    uint64(i),
				Value:      []byte{},
			})
		}

		err := store.append(scs, sb)
		require.NoError(t, err)
	}

	entries, err := store.getAll(iids[k-1], sb.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, l*store.maxNbrBlock, len(entries))
	require.Equal(t, n/l-store.maxNbrBlock, entries[0].BlockIndex)
}

func TestStateChangeStorage_Race(t *testing.T) {
	store, name := generateDB(t)
	defer os.Remove(name)

	store.setMaxSize(100)

	sb1 := createBlock()
	sb2 := createBlock()
	sb3 := createBlock()
	scs := generateStateChanges(1000)

	wg := sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)

		go func() {
			store.append(scs, sb1)
			store.append(scs, sb2)
			store.append(scs, sb3)

			wg.Done()
		}()
	}

	wg.Wait()
}

func generateStateChanges(n int) StateChanges {
	id := genID().Slice()

	scs := StateChanges{}
	for i := 0; i < n; i++ {
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
	require.NoError(t, err)
	tmpDB.Close()

	db, err := bbolt.Open(tmpDB.Name(), 0600, nil)
	require.NoError(t, err)

	scs := stateChangeStorage{db: db, bucket: []byte("scstest")}
	db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket(scs.bucket)
		return err
	})

	return &scs, tmpDB.Name()
}

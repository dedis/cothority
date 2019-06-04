package byzcoin

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3/util/random"
)

func TestInstanceIDCache(t *testing.T) {
	r := random.New()
	st, err := newMemStateTrie([]byte("xyz"))
	require.NoError(t, err)

	// Write some values to the state trie and check that the cache builds
	// it correctly.
	n := 5
	iids := make([][]byte, n)
	dids := make([][]byte, n)
	for i := range iids {
		iids[i] = make([]byte, 32)
		random.Bytes(iids[i], r)

		dids[i] = make([]byte, 32)
		random.Bytes(dids[i], r)
	}

	scs := []StateChange{
		{
			StateAction: Create,
			InstanceID:  iids[0],
			ContractID:  "dummy_contract",
			Value:       iids[0],
			DarcID:      dids[0],
			Version:     0,
		},
		{
			StateAction: Create,
			InstanceID:  iids[1],
			ContractID:  "dummy_contract",
			Value:       iids[1],
			DarcID:      dids[1],
			Version:     0,
		},
		{
			StateAction: Create,
			InstanceID:  iids[2],
			ContractID:  "dummy_contract_2",
			Value:       iids[2],
			DarcID:      dids[2],
			Version:     0,
		},
		{
			StateAction: Update,
			InstanceID:  iids[2],
			ContractID:  "dummy_contract_2",
			Value:       []byte{},
			DarcID:      dids[2],
			Version:     1,
		},
	}
	require.NoError(t, st.StoreAll(scs, 1))

	cache := newInstanceIDCache()
	require.NoError(t, cache.rebuild(st))
	require.Equal(t, 2, len(cache.get("dummy_contract")))
	require.Equal(t, 1, len(cache.get("dummy_contract_2")))

	// Create new state changes that spawn/delete instances and check the
	// cache.
	newScs := []StateChange{
		{
			StateAction: Create,
			InstanceID:  iids[3],
			ContractID:  "dummy_contract_2",
			Value:       iids[3],
			DarcID:      dids[3],
			Version:     0,
		},
		{
			StateAction: Update,
			InstanceID:  iids[1],
			ContractID:  "dummy_contract",
			Value:       []byte{},
			DarcID:      dids[1],
			Version:     0,
		},
		{
			StateAction: Remove,
			InstanceID:  iids[0],
			ContractID:  "dummy_contract",
			Value:       iids[0],
			DarcID:      dids[0],
			Version:     0,
		},
	}
	require.NoError(t, cache.update(newScs))
	require.Equal(t, 1, len(cache.get("dummy_contract")))
	require.Equal(t, 2, len(cache.get("dummy_contract_2")))

	// Check that empty contract IDs are not added during rebuild.
	emptyCID := StateChange{
		StateAction: Create,
		InstanceID:  iids[4],
		ContractID:  "",
		Value:       iids[4],
		DarcID:      dids[4],
		Version:     0,
	}
	require.NoError(t, st.StoreAll([]StateChange{emptyCID}, 2))
	require.NoError(t, cache.rebuild(st))
	require.Equal(t, 2, len(cache.get("dummy_contract")))
	require.Equal(t, 1, len(cache.get("dummy_contract_2")))
	require.Equal(t, 0, len(cache.get("")))

	// Check that empty contract IDs are not added during udpate.
	require.NoError(t, cache.update([]StateChange{emptyCID}))
	require.Equal(t, 2, len(cache.get("dummy_contract")))
	require.Equal(t, 1, len(cache.get("dummy_contract_2")))
	require.Equal(t, 0, len(cache.get("")))
}

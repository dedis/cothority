package byzcoin

import (
	"testing"

	"github.com/dedis/cothority/darc"
	"github.com/stretchr/testify/require"
)

// TestStateTrie is a sanity check for setting and retrieving keys, values and
// index. The main functionalities are tested in the trie package.
func TestStateTrie(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	st, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, st)
	require.NotEqual(t, -1, st.GetIndex())

	key := []byte("testInstance")
	contractID := []byte("testContract")
	value := []byte("testValue")
	version := uint64(123)
	darcID := darc.ID([]byte("123"))
	sc := StateChange{
		StateAction: Create,
		InstanceID:  key,
		ContractID:  contractID,
		Value:       value,
		Version:     version,
		DarcID:      darcID,
	}
	// store with bad expected root hash should fail, value should not be inside
	require.Error(t, st.VerifiedStoreAll([]StateChange{sc}, 5, []byte("badhash")))
	_, _, _, _, err = st.GetValues(key)
	require.Equal(t, errKeyNotSet, err)

	// store the state changes normally using StoreAll and it should work
	require.NoError(t, st.StoreAll([]StateChange{sc}, 5))
	require.Equal(t, st.GetIndex(), 5)

	require.NoError(t, st.StoreAll([]StateChange{sc}, 6))
	require.Equal(t, st.GetIndex(), 6)

	_, _, _, _, err = st.GetValues(append(key, byte(0)))
	require.Equal(t, errKeyNotSet, err)

	val, ver, cid, did, err := st.GetValues(key)
	require.Equal(t, value, val)
	require.Equal(t, version, ver)
	require.Equal(t, cid, string(contractID))
	require.True(t, did.Equal(darcID))
}

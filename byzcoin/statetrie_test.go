package byzcoin

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"
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
	contractID := "testContract"
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
	require.Error(t, st.VerifiedStoreAll([]StateChange{sc}, 5, CurrentVersion, []byte("badhash")))
	_, _, _, _, err = st.GetValues(key)
	require.Equal(t, errKeyNotSet, err)

	// store the state changes normally using StoreAll and it should work
	require.NoError(t, st.StoreAll([]StateChange{sc}, 5, CurrentVersion))
	require.Equal(t, st.GetIndex(), 5)

	require.NoError(t, st.StoreAll([]StateChange{sc}, 6, CurrentVersion))
	require.Equal(t, st.GetIndex(), 6)

	_, _, _, _, err = st.GetValues(append(key, byte(0)))
	require.Equal(t, errKeyNotSet, err)

	val, ver, cid, did, err := st.GetValues(key)
	require.Equal(t, value, val)
	require.Equal(t, version, ver)
	require.Equal(t, cid, string(contractID))
	require.True(t, did.Equal(darcID))

	// test the staging state trie, most of the tests are done in the trie package
	key2 := []byte("key2")
	val2 := []byte("val2")
	sst := st.MakeStagingStateTrie()
	oldRoot := sst.GetRoot()
	require.NoError(t, sst.Set(key2, val2))
	newRoot := sst.GetRoot()
	require.False(t, bytes.Equal(oldRoot, newRoot))
	candidateVal2, err := sst.Get(key2)
	require.NoError(t, err)
	require.True(t, bytes.Equal(val2, candidateVal2))

	// test the commit of staging state trie, root should be computed differently now, but it should be the same
	require.NoError(t, sst.Commit())
	require.True(t, bytes.Equal(sst.GetRoot(), newRoot))
}

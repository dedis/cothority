package byzcoin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testContractFn(in []byte) (Contract, error) {
	return nil, nil
}

// Test basic usage of the registry.
func TestContracts_Registry(t *testing.T) {
	r := newContractRegistry()
	r.register("a", testContractFn, false)

	f, exists := r.Search("a")
	require.NotNil(t, f)
	require.True(t, exists)

	require.Error(t, r.register("a", testContractFn, false))

	f, exists = r.Search("b")
	require.Nil(t, f)
	require.False(t, exists)

	r2 := r.clone()
	require.True(t, r.locked)
	require.True(t, r2.locked)

	f, exists = r2.Search("a")
	require.NotNil(t, f)
	require.True(t, exists)

	require.Error(t, r.register("c", testContractFn, false))
	require.NoError(t, r.register("c", testContractFn, true))
}

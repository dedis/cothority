package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateChangeCache(t *testing.T) {
	cache := newStateChangeCache()
	require.NotNil(t, cache.cache)

	scID := []byte("scID")
	digest := []byte("digest")

	_, _, _, err := cache.get(scID, digest)
	require.Error(t, err)

	root := []byte("root")
	txs := ClientTransactions([]ClientTransaction{})
	scs := StateChanges([]StateChange{})
	cache.update(scID, digest, root, txs, scs)

	root1, txs1, scs1, err := cache.get(scID, digest)
	require.NoError(t, err)
	require.Equal(t, root, root1)
	require.Equal(t, txs, txs1)
	require.Equal(t, scs, scs1)
}

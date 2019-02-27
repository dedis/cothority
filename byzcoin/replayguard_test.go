package byzcoin

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"
)

func TestReplayGuard(t *testing.T) {
	// increment
	sst, err := newMemStagingStateTrie([]byte("my nonce"))
	require.NoError(t, err)
	signers := []darc.Signer{darc.NewSignerEd25519(nil, nil), darc.NewSignerEd25519(nil, nil)}
	var ids []darc.Identity
	for _, signer := range signers {
		ids = append(ids, signer.Identity())
	}
	scs, err := incrementSignerCounters(sst, ids)
	require.NoError(t, err)
	require.NoError(t, sst.StoreAll(scs))

	// checke that the state changes use the Create action
	for _, sc := range scs {
		require.Equal(t, Create, sc.StateAction)
	}

	// check that they're 1 using getSignerCounter
	ctr0, err := getSignerCounter(sst, signers[0].Identity().String())
	require.NoError(t, err)
	require.Equal(t, uint64(1), ctr0)

	ctr1, err := getSignerCounter(sst, signers[1].Identity().String())
	require.NoError(t, err)
	require.Equal(t, uint64(1), ctr1)

	// increment again, now the counter state is at 2
	scs, err = incrementSignerCounters(sst, ids)
	require.NoError(t, err)
	require.NoError(t, sst.StoreAll(scs))

	// check that state changes use the Update action
	for _, sc := range scs {
		require.Equal(t, Update, sc.StateAction)
	}

	// verify, the new counter state must be 3
	err = verifySignerCounters(sst, []uint64{3, 3}, ids)
	require.NoError(t, err)
}

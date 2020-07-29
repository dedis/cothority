package byzcoin

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestNewROSTSimul(t *testing.T) {
	rs := NewROSTSimul()

	value := uint64(100)
	name := "test"
	coinID, err := rs.CreateCoin(name, value)
	require.NoError(t, err)
	coin, err := rs.GetCoin(coinID)
	require.NoError(t, err)
	require.Equal(t, value, coin.Value)
	require.True(t, strings.HasPrefix(string(coin.Name[:]), name))

	_, ver, cid, _, err := rs.GetValues(coinID[:])
	require.NoError(t, err)
	require.Equal(t, uint64(0), ver)
	require.Equal(t, "coin", cid)

	require.NoError(t, rs.SetCoin(coinID, 2*value))
	coin, err = rs.GetCoin(coinID)
	require.NoError(t, err)
	require.Equal(t, 2*value, coin.Value)

	_, _, err = rs.WithdrawCoin(coinID, value)
	require.NoError(t, err)

	_, _, err = rs.WithdrawCoin(coinID, 2*value)
	require.Error(t, err)
}

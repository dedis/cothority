package byzcoin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHeartbeat_Start(t *testing.T) {
	hb := newHeartbeats()
	defer hb.closeAll()

	timeoutChan := make(chan string, 1)
	k1 := "k1"
	require.NoError(t, hb.start(k1, time.Second, timeoutChan))
	require.True(t, hb.exists(k1))
	require.False(t, hb.exists("zz"))

	// cannot start it again
	require.Error(t, hb.start(k1, time.Second, timeoutChan))

	// can start a different one
	k2 := "k2"
	require.NoError(t, hb.start(k2, time.Second, timeoutChan))
	require.True(t, hb.exists(k2))
	require.Equal(t, len(hb.heartbeatMap), 2)
}

func TestHeartbeat_Timeout(t *testing.T) {
	hb := newHeartbeats()
	defer hb.closeAll()

	timeoutChan := make(chan string, 1)
	k1 := "k1"
	require.NoError(t, hb.start(k1, time.Millisecond, timeoutChan))
	require.Error(t, hb.beat("zz"))
	require.NoError(t, hb.beat(k1))

	select {
	case k := <-timeoutChan:
		require.Equal(t, k, k1)
	case <-time.After(200 * time.Millisecond):
		require.Fail(t, "did not get message in timeoutChan")
	}

	// Wait for a bit and beat again, we expect the latest heartbeat to
	// change.

	lastBeat, err := hb.getLatestHeartbeat(k1)
	require.NoError(t, err)

	time.Sleep(time.Millisecond)
	require.NoError(t, hb.beat(k1))

	newBeat, err := hb.getLatestHeartbeat(k1)
	require.NoError(t, err)
	if !newBeat.After(lastBeat) {
		require.Fail(t, "heartbeat was not updated")
	}
}

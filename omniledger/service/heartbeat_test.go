package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHeartbeat_Start(t *testing.T) {
	hb := newHeartbeats()
	defer hb.closeAll()

	toChan := make(chan string, 1)
	k1 := "k1"
	require.NoError(t, hb.start(k1, time.Second, toChan))
	require.True(t, hb.exists(k1))
	require.False(t, hb.exists("zz"))

	// cannot start it again
	require.Error(t, hb.start(k1, time.Second, toChan))

	// can start a different one
	k2 := "k2"
	require.NoError(t, hb.start(k2, time.Second, toChan))
	require.True(t, hb.exists(k2))
}

func TestHeartbeat_Timeout(t *testing.T) {
	hb := newHeartbeats()
	defer hb.closeAll()

	toChan := make(chan string, 1)
	k1 := "k1"
	require.NoError(t, hb.start(k1, time.Millisecond, toChan))
	require.Error(t, hb.beat("zz"))

	expected := time.Now()
	require.NoError(t, hb.beat(k1))

	time.Sleep(2 * time.Millisecond)

	select {
	case k := <-toChan:
		require.Equal(t, k, k1)
	default:
		require.Fail(t, "did not get message in toChan")
	}

	lastBeat, err := hb.getLatestHeartbeat(k1)
	require.NoError(t, err)
	if lastBeat.After(expected.Add(time.Millisecond/2)) || lastBeat.Before(expected.Add(-time.Microsecond/2)) {
		require.Fail(t, "lastBeat is not within a millisecond of the expected range")
	}

	// if we beat again, then the latest heartbeat should be updated
	time.Sleep(2 * time.Millisecond)
	expected = time.Now()
	require.NoError(t, hb.beat(k1))

	lastBeat, err = hb.getLatestHeartbeat(k1)
	require.NoError(t, err)
	if lastBeat.After(expected.Add(time.Millisecond/2)) || lastBeat.Before(expected.Add(-time.Microsecond/2)) {
		require.Fail(t, "lastBeat is not within a millisecond of the expected range")
	}
}

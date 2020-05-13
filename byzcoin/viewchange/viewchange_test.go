package viewchange

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/skipchain"
)

func TestViewChange_Normal1(t *testing.T) {
	testNormal(t, 1)
}

func TestViewChange_Normal2(t *testing.T) {
	testNormal(t, 2)
}

func TestViewChange_Timeout1(t *testing.T) {
	testTimeout(t, 1)
}

func TestViewChange_Timeout2(t *testing.T) {
	testTimeout(t, 2)
}

// testSetupViewChangeF1 sets up the view-change log and sends f view-change
// messages. If anomaly is set then it sends one more message to the anomaly
// channel.
func testSetupViewChangeF1(t *testing.T, signerID [16]byte, dur time.Duration, f int, anomaly bool) (chan bool, chan bool, View, *Controller) {
	view := View{
		ID:          skipchain.SkipBlockID([]byte{42}),
		LeaderIndex: 1,
	}
	vcChan := make(chan bool, 1)
	nvChan := make(chan bool, 1)
	vcF := func(view View) error {
		vcChan <- true
		return nil
	}
	nvF := func(proof []InitReq) {
		nvChan <- true
	}
	vcl := NewController(vcF, nvF, func(v View) bool { return true })
	go vcl.Start(signerID, []byte{}, dur, 2*f+1)

	// We receive view-change requests and send our own because we detected
	// an anomaly.
	for i := 0; i < f; i++ {
		req := InitReq{
			SignerID: [16]byte{byte(i)},
			View:     view,
		}
		vcl.AddReq(req)
	}
	if anomaly {
		vcl.AddReq(InitReq{
			SignerID: signerID,
			View:     view,
		})
		select {
		case <-vcChan:
		case <-time.After(10 * time.Millisecond):
			require.Fail(t, "view change function should have been called")
		}
	}
	return vcChan, nvChan, view, &vcl
}

// testSetupViewChange2F1 sets up the view-change log and sends 2f+1 messages
// including one from myself. Hence by the time this function returns the timer
// should have started and the state should be at startedTimerState.
func testSetupViewChange2F1(t *testing.T, signerID [16]byte, dur time.Duration, f int) (chan bool, chan bool, View, *Controller) {
	vcChan, nvChan, view, vcl := testSetupViewChangeF1(t, signerID, dur, f, true)
	// Suppose more view-change message arrive, until there are 2f+1 of
	// them, then a timer should start.
	for i := f + 1; i < 2*f+1; i++ {
		req := InitReq{
			SignerID: [16]byte{byte(i)},
			View:     view,
		}
		vcl.AddReq(req)
	}
	select {
	case ctr := <-vcl.startTimerChan:
		require.Equal(t, 1, ctr)
	case <-time.After(dur):
		require.Fail(t, "timer should have started")
	}
	return vcChan, nvChan, view, vcl
}

func testNormal(t *testing.T, f int) {
	dur := 100 * time.Millisecond
	mySignerID := [16]byte{byte(255)}
	_, _, view, vcl := testSetupViewChange2F1(t, mySignerID, dur, f)
	defer vcl.Stop()

	// Check that view-change is in progress.
	require.True(t, vcl.Waiting())

	// If we signal that the view-change completed successfully, then
	// everything should be reset.
	vcl.Done(view)
	select {
	case ctr := <-vcl.stopTimerChan:
		require.Equal(t, 0, ctr)
	case <-time.After(dur):
		require.Fail(t, "timer should have stopped on done")
	}

	// Check that view-change is finsihed.
	require.False(t, vcl.Waiting())
}

func testTimeout(t *testing.T, f int) {
	dur := 100 * time.Millisecond
	mySignerID := [16]byte{byte(255)}
	vcChan, nvChan, view, vcl := testSetupViewChange2F1(t, mySignerID, dur, f)
	defer vcl.Stop()

	// If the view-change did not complete successfully, the timer should
	// expire and we should move to the next view.
	select {
	case <-nvChan:
	case <-time.After(10 * time.Millisecond):
		require.Fail(t, "new view function should have been called")
	}
	select {
	case ctr := <-vcl.expireTimerChan:
		require.Equal(t, 1, ctr)
	case <-time.After(2*dur + dur/2):
		require.Fail(t, "expected timer to expire")
	}

	// Add more requests to trigger the second timer. We only need to add
	// 2*f because the anomaly request is automatically send upon timer
	// expiration.
	newView := view
	newView.LeaderIndex++
	for i := 0; i < 2*f; i++ {
		req := InitReq{
			SignerID: [16]byte{byte(i)},
			View:     newView,
		}
		vcl.AddReq(req)
	}
	select {
	case <-vcChan:
	case <-time.After(100 * time.Millisecond):
		require.Fail(t, "view change function should have been called")
	}
	select {
	case ctr := <-vcl.startTimerChan:
		require.Equal(t, newView.LeaderIndex, ctr)
	case <-time.After(dur + dur/2):
		require.Fail(t, "timer should have started")
	}

	// Another timer should expire if we do not see anything for 4*dur.
	select {
	case <-nvChan:
	case <-time.After(10 * time.Millisecond):
		require.Fail(t, "new view function should have been called")
	}
	select {
	case i := <-vcl.expireTimerChan:
		require.Equal(t, 2, i)
	case <-time.After(4*dur + dur/2):
		require.Fail(t, "expected timer to expire")
	}
}

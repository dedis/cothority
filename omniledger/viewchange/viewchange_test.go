package viewchange

import (
	"testing"
	"time"

	"github.com/dedis/cothority/skipchain"
	"github.com/stretchr/testify/require"
)

// testSetupViewChangeF1 sets up the view-change log and sends f view-change
// messages. If anomaly is set then it sends one more message to the anomaly
// channel.
func testSetupViewChangeF1(t *testing.T, signerID [16]byte, dur time.Duration, f int, anomaly bool) (chan bool, View, *Controller) {
	view := View{
		ID:          skipchain.SkipBlockID([]byte{42}),
		LeaderIndex: 1,
	}
	vcChan := make(chan bool, 1)
	vcF := func(view View) error {
		vcChan <- true
		return nil
	}
	nvF := func(proof []InitReq) {
	}
	vcl := NewController(vcF, nvF, func(v View) bool { return true })
	go vcl.Start(signerID, []byte{}, dur, f)

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
		vcl.AddAnomaly(InitReq{
			SignerID: signerID,
			View:     view,
		})
		select {
		case <-vcChan:
		case <-time.After(10 * time.Millisecond):
			require.Fail(t, "view change function should have been called")
		}
	}
	return vcChan, view, &vcl
}

// testSetupViewChange2F1 sets up the view-change log and sends 2f+1 messages
// including one from myself. Hence by the time this function returns the timer
// should have started and the state should be at startedTimerState.
func testSetupViewChange2F1(t *testing.T, signerID [16]byte, dur time.Duration, f int) (chan bool, View, *Controller) {
	vcChan, view, vcl := testSetupViewChangeF1(t, signerID, dur, f, true)
	// Suppose more view-change message arrive, until there are 2f+1 of
	// them, then a timer should start.
	ctrChan := vcl.diagnoseStartTimer(func() {
		for i := f + 1; i < 2*f+1; i++ {
			req := InitReq{
				SignerID: [16]byte{byte(i)},
				View:     view,
			}
			vcl.AddReq(req)
		}
	})
	select {
	case ctr := <-ctrChan:
		require.Equal(t, 1, ctr)
	case <-time.After(dur):
		require.Fail(t, "timer should have started")
	}
	return vcChan, view, vcl
}

func TestViewChange_Normal(t *testing.T) {
	dur := 100 * time.Millisecond
	f := 2
	mySignerID := [16]byte{byte(255)}
	_, view, vcl := testSetupViewChange2F1(t, mySignerID, dur, f)
	defer vcl.Stop()

	// If we signal that the view-change completed successfully, then
	// everything should be reset.
	ctrChan := vcl.diagnoseStopTimer(func() {
		vcl.Done(view)
	})
	select {
	case ctr := <-ctrChan:
		require.Equal(t, 0, ctr)
	case <-time.After(dur):
		require.Fail(t, "timer should have stopped on done")
	}
}

func TestViewChange_Timeout(t *testing.T) {
	dur := 100 * time.Millisecond
	f := 2
	mySignerID := [16]byte{byte(255)}
	vcChan, view, vcl := testSetupViewChange2F1(t, mySignerID, dur, f)
	defer vcl.Stop()

	// If the view-change did not complete successfully, the timer should
	// expire and we should move to the next view.
	select {
	case ctr := <-vcl.diagnoseExpireTimer():
		require.Equal(t, 1, ctr)
	case <-time.After(2*dur + dur/2):
		require.Fail(t, "expected timer to expire")
	}

	// Add more requests to trigger the second timer. We only need to add
	// 2*f because the anomaly request is automatically send upon timer
	// expiration.
	newView := view
	newView.LeaderIndex++
	startCtrChan := vcl.diagnoseStartTimer(func() {
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
	})
	select {
	case ctr := <-startCtrChan:
		require.Equal(t, newView.LeaderIndex, ctr)
	case <-time.After(dur + dur/2):
		require.Fail(t, "timer should have started")
	}

	// Another timer should expire if we do not see anything for 4*dur.
	select {
	case i := <-vcl.diagnoseExpireTimer():
		require.Equal(t, 2, i)
	case <-time.After(4*dur + dur/2):
		require.Fail(t, "expected timer to expire")
	}
}

// TestViewChange_AutoStart checks that we can start view-change not from
// sending in an anomaly (e.g., detected a timeout) but from receiving many
// view-change messages from other nodes.
func TestViewChange_AutoStart(t *testing.T) {
	dur := 100 * time.Millisecond
	f := 2
	mySignerID := [16]byte{byte(255)}
	vcChan, view, vcl := testSetupViewChangeF1(t, mySignerID, dur, f, false)
	defer vcl.Stop()
	// Send a in regular request instead of an anomaly should trigger the
	// anomaly case.
	vcl.AddReq(InitReq{
		SignerID: [16]byte{byte(f)},
		View:     view,
	})
	select {
	case <-vcChan:
	case <-time.After(10 * time.Millisecond):
		require.Fail(t, "view change function should have been called")
	}
}

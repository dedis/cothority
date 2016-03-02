package monitor

import (
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/testutil"
)

func TestProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["servers"] = "1"
	m["hosts"] = "1"
	m["filter_round"] = "100"
	stat := NewStats(m)
	fresh := stat.String()
	// First set up monitor listening
	monitor := NewMonitor(stat)
	monitor.SinkPort = 8000
	done := make(chan bool)
	go func() {
		monitor.Listen()
		done <- true
	}()
	time.Sleep(100 * time.Millisecond)
	// Then setup proxy
	// change port so the proxy does not listen to the same
	// than the original monitor

	// proxy listens to 0.0.0.0:8000 & redirects to
	// localhost:10000 (DefaultSinkPort)
	go Proxy("localhost:" + strconv.Itoa(DefaultSinkPort))

	time.Sleep(100 * time.Millisecond)
	// Then measure
	proxyAddr := "localhost:" + strconv.Itoa(monitor.SinkPort)
	err := ConnectSink(proxyAddr)
	if err != nil {
		t.Errorf("Can not connect to proxy : %s", err)
		return
	}

	meas := NewMeasure("setup")
	meas.Measure()
	time.Sleep(100 * time.Millisecond)
	meas.Measure()

	s, err := GetReady(proxyAddr)
	if err != nil {
		t.Error("Couldn't get stats from proxy")
	}
	if s.Ready != 0 {
		t.Error("stats.Ready should be 0")
	}
	Ready(proxyAddr)
	s, err = GetReady(proxyAddr)
	if err != nil {
		t.Error("Couldn't get stats from proxy")
	}
	if s.Ready != 1 {
		t.Error("stats.Ready should be 1")
	}

	EndAndCleanup()

	select {
	case <-done:
		s := monitor.Stats()
		s.Collect()
		if s.String() == fresh {
			t.Error("stats not updated?")
		}
		return
	case <-time.After(2 * time.Second):
		t.Error("Monitor not finished")
	}
}

func TestReadyProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["servers"] = "1"
	m["hosts"] = "1"
	stat := NewStats(m)
	// First set up monitor listening
	monitor := NewMonitor(stat)
	monitor.SinkPort = 8000
	done := make(chan bool)
	go func() {
		if err := monitor.Listen(); err != nil {
			dbg.Error("Could not start listener:", err)
			dbg.Error("Retry once in 500ms ...")
			time.Sleep(500 * time.Millisecond)
			if err := monitor.Listen(); err != nil {
				dbg.Fatal("Could not start listener:", err)
			}
		}

		done <- true
	}()
	time.Sleep(100 * time.Millisecond)
	// Then setup proxy
	// change port so the proxy does not listen to the same
	// than the original monitor

	// proxy listen to 0.0.0.0:8000 & redirect to
	// localhost:4000

	go Proxy("localhost:" + strconv.Itoa(DefaultSinkPort))

	// wait for proxy to start:
	time.Sleep(100 * time.Millisecond)
	// Then measure

	proxyAddr := "localhost:" + strconv.Itoa(monitor.SinkPort)
	if err := ConnectSink(proxyAddr); err != nil {
		dbg.Error("Could not connect to proxy:", err)
		dbg.Error("Retry once in 500ms ...")
		time.Sleep(500 * time.Millisecond)
		if err := ConnectSink(proxyAddr); err != nil {
			t.Errorf("Can not connect to proxy : %s", err)
			return
		}
	}

	s, err := GetReady(proxyAddr)
	if err != nil {
		t.Error("Couldn't get stats from proxy")
	}
	if s.Ready != 0 {
		t.Error("stats.Ready should be 0")
	}
	Ready(proxyAddr)
	s, err = GetReady(proxyAddr)
	if err != nil {
		t.Error("Couldn't get stats from proxy")
	}
	if s.Ready != 1 {
		t.Error("stats.Ready should be 1")
	}

	EndAndCleanup()

	select {
	case <-done:
		return
	case <-time.After(2 * time.Second):
		t.Error("Monitor not finished")
	}
}

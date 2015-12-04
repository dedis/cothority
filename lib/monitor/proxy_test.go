package monitor

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"testing"
	"time"
)

func TestProxy(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["ppm"] = "1"
	m["filter_round"] = "100"
	stat := NewStats(m)
	fresh := stat.String()
	// First set up monitor listening
	monitor := NewMonitor(stat)
	done := make(chan bool)
	go func() {
		monitor.Listen()
		done <- true
	}()
	time.Sleep(100 * time.Millisecond)
	// Then setup proxy
	// change port so the proxy does not listen to the same
	// than the original monitor
	oldSink := SinkPort
	SinkPort = 8000
	// proxy listen to 0.0.0.0:8000 & redirect to
	// localhost:4000
	go Proxy("localhost:" + strconv.Itoa(oldSink))

	time.Sleep(100 * time.Millisecond)
	// Then measure
	proxyAddr := "localhost:" + strconv.Itoa(SinkPort)
	err := ConnectSink(proxyAddr)
	if err != nil {
		t.Error(fmt.Sprintf("Can not connect to proxy : %s", err))
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

	SinkPort = oldSink
	End()
	StopSink()
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
	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["ppm"] = "1"
	stat := NewStats(m)
	// First set up monitor listening
	monitor := NewMonitor(stat)
	done := make(chan bool)
	go func() {
		monitor.Listen()
		done <- true
	}()
	time.Sleep(100 * time.Millisecond)
	// Then setup proxy
	// change port so the proxy does not listen to the same
	// than the original monitor
	oldSink := SinkPort
	SinkPort = 8000
	// proxy listen to 0.0.0.0:8000 & redirect to
	// localhost:4000
	go Proxy("localhost:" + oldSink)

	time.Sleep(100 * time.Millisecond)
	// Then measure
	proxyAddr := "localhost:" + strconv.Itoa(SinkPort)
	err := ConnectSink(proxyAddr)
	if err != nil {
		t.Error(fmt.Sprintf("Can not connect to proxy : %s", err))
		return
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

	SinkPort = oldSink
	End()
	StopSink()
}

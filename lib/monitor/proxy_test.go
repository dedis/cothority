package monitor

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

func TestProxy(t *testing.T) {
	//	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["hosts"] = "1"
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
	oldSink := DefaultSinkPort
	DefaultSinkPort = 8000
	// proxy listen to 0.0.0.0:8000 & redirect to
	// localhost:4000
	go Proxy("localhost:" + strconv.Itoa(oldSink))

	time.Sleep(100 * time.Millisecond)
	// Then measure
	proxyAddr := "localhost:" + strconv.Itoa(DefaultSinkPort)
	err := ConnectSink(proxyAddr)
	if err != nil {
		t.Error(fmt.Sprintf("Can not connect to proxy : %s", err))
		return
	}

	meas := NewSingleMeasure("setup", 10)
	meas.Record()
	time.Sleep(100 * time.Millisecond)
	meas = NewSingleMeasure("setup", 20)
	meas.Record()
	DefaultSinkPort = oldSink
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

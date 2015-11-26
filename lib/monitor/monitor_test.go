package monitor

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"testing"
	"time"
)

func TestProxy(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 2)
	m := make(map[string]string)
	m["machines"] = "1"
	m["ppm"] = "1"
	stat := NewStats(m)
	fresh := stat.String()
	// First set up monitor listening
	go Monitor(stat)
	time.Sleep(100 * time.Millisecond)
	// Then setup proxy
	// change port so the proxy does not listen to the same
	// than the original monitor
	oldSink := SinkPort
	SinkPort = "8000"
	// proxy listen to 0.0.0.0:8000 & redirect to
	// localhost:4000
	go Proxy("localhost:" + oldSink)

	time.Sleep(100 * time.Millisecond)
	// Then measure
	err := ConnectSink("localhost:" + SinkPort)
	if err != nil {
		t.Error(fmt.Sprintf("Can not connect to proxy : %s", err))
		return
	}

	meas := NewMeasure("round")
	meas.Measure()
	time.Sleep(200 * time.Millisecond)
	meas.Measure()
	End()
	time.Sleep(100 * time.Millisecond)
	updated := stat.String()
	if updated == fresh {
		t.Error("Stats not updated ?")
	}
	SinkPort = oldSink
	Stop()
	StopSink()
}

func TestMonitor(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 2)
	m := make(map[string]string)
	m["machines"] = "1"
	m["ppm"] = "1"
	stat := NewStats(m)
	fresh := stat.String()
	// First set up monitor listening
	go Monitor(stat)
	time.Sleep(100 * time.Millisecond)

	// Then measure
	err := ConnectSink("localhost:" + SinkPort)
	if err != nil {
		t.Error(fmt.Sprintf("Error starting monitor: %s", err))
		return
	}

	meas := NewMeasure("round")
	meas.Measure()
	time.Sleep(200 * time.Millisecond)
	meas.Measure()
	End()
	time.Sleep(100 * time.Millisecond)
	updated := stat.String()
	if updated == fresh {
		t.Error("Stats not updated ?")
	}

	Stop()
	StopSink()
}

func TestReady(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["ppm"] = "1"
	stat := NewStats(m)
	// First set up monitor listening
	go Monitor(stat)
	time.Sleep(100 * time.Millisecond)
	host := "localhost:" + SinkPort

	s, err := GetReady(host)
	if err != nil{
		t.Fatal("Couldn't get number of peers:", err)
	}
	if s.Ready != 0{
		t.Fatal("Stats.Ready != 0")
	}

	err = Ready(host)
	if err != nil {
		t.Errorf("Error starting monitor: %s", err)
		return
	}

	s, err = GetReady(host)
	if err != nil{
		t.Fatal("Couldn't get number of peers:", err)
	}
	if s.Ready != 1{
		t.Fatal("Stats.Ready != 1")
	}

	Stop()
	StopSink()
}

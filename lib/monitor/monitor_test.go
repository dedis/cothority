package monitor

import (
	"bytes"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"testing"
	"time"
)

// setupMonitor launches a basic monitor with a created Stats object
// When finished with the monitor, just call `End()`
func setupMonitor(t *testing.T) (*Monitor, *Stats) {
	dbg.TestOutput(testing.Verbose(), 2)
	m := make(map[string]string)
	m["machines"] = "1"
	m["ppm"] = "1"
	stat := NewStats(m)
	// First set up monitor listening
	mon := NewMonitor(stat)
	go mon.Listen()
	time.Sleep(100 * time.Millisecond)

	// Then measure
	err := ConnectSink("localhost:" + strconv.Itoa(SinkPort))
	if err != nil {
		t.Fatal(fmt.Sprintf("Error starting monitor: %s", err))
	}
	return mon, stat
}

func TestMonitor(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 2)
	m := make(map[string]string)
	m["machines"] = "1"
	m["ppm"] = "1"
	stat := NewStats(m)
	fresh := stat.String()
	// First set up monitor listening
	mon := NewMonitor(stat)
	go mon.Listen()
	time.Sleep(100 * time.Millisecond)

	// Then measure
	err := ConnectSink("localhost:" + strconv.Itoa(SinkPort))
	if err != nil {
		t.Error(fmt.Sprintf("Error starting monitor: %s", err))
		return
	}

	meas := NewSingleMeasure("round", 10)
	meas.Record()
	time.Sleep(200 * time.Millisecond)
	NewSingleMeasure("round", 20)
	End()
	time.Sleep(100 * time.Millisecond)
	updated := stat.String()
	if updated == fresh {
		t.Error("Stats not updated ?")
	}

}

func TestKeyOrder(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["hosts"] = "1"
	m["bf"] = "2"
	// create stats
	stat := NewStats(m)
	m1 := NewSingleMeasure("round", 10)
	m2 := NewSingleMeasure("setup", 5)
	stat.Update(m1)
	stat.Update(m2)
	str := new(bytes.Buffer)
	stat.WriteHeader(str)
	stat.WriteValues(str)

	stat2 := NewStats(m)
	stat2.Update(m1)
	stat2.Update(m2)

	str2 := new(bytes.Buffer)
	stat2.WriteHeader(str2)
	stat2.WriteValues(str2)
	if !bytes.Equal(str.Bytes(), str2.Bytes()) {
		t.Fatal("KeyOrder / output not the same for same stats")
	}
}

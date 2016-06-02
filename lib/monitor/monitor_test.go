package monitor

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

// setupMonitor launches a basic monitor with a created Stats object
// When finished with the monitor, just call `End()`
func setupMonitor(t *testing.T) (*Monitor, *Stats) {
	dbg.TestOutput(testing.Verbose(), 2)
	m := make(map[string]string)
	m["servers"] = "1"
	stat := NewStats(m)
	// First set up monitor listening
	mon := NewMonitor(stat)
	go mon.Listen()
	time.Sleep(100 * time.Millisecond)

	// Then measure
	err := ConnectSink("localhost:" + strconv.Itoa(mon.SinkPort))
	EnableMeasure(true)
	if err != nil {
		t.Fatal(fmt.Sprintf("Error starting monitor: %s", err))
	}
	return mon, stat
}

func TestReadyNormal(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["servers"] = "1"
	stat := NewStats(m)
	fresh := stat.String()
	// First set up monitor listening
	mon := NewMonitor(stat)
	go mon.Listen()
	time.Sleep(100 * time.Millisecond)

	// Then measure
	err := ConnectSink("localhost:" + strconv.Itoa(DefaultSinkPort))
	if err != nil {
		t.Fatal(fmt.Sprintf("Error starting monitor: %s", err))
		return
	}

	meas := NewSingleMeasure("round", 10)
	meas.Record()
	time.Sleep(200 * time.Millisecond)
	NewSingleMeasure("round", 20)
	EndAndCleanup()
	time.Sleep(100 * time.Millisecond)
	updated := mon.Stats().String()
	if updated == fresh {
		t.Fatal("Stats not updated ?")
	}

}

func TestKeyOrder(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["servers"] = "1"
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

package monitor

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

func TestMonitor(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 2)
	m := make(map[string]string)
	m["servers"] = "1"
	m["hosts"] = "1"
	stat := NewStats(m)
	fresh := stat.String()
	// First set up monitor listening
	mon := NewMonitor(stat)
	go mon.Listen()
	time.Sleep(100 * time.Millisecond)

	// Then measure
	err := ConnectSink("localhost:" + strconv.Itoa(DefaultSinkPort))
	if err != nil {
		t.Error(fmt.Sprintf("Error starting monitor: %s", err))
		return
	}

	meas := NewMeasure("round")
	meas.Measure()
	time.Sleep(200 * time.Millisecond)
	meas.Measure()
	EndAndCleanup()
	time.Sleep(100 * time.Millisecond)
	updated := stat.String()
	if updated == fresh {
		t.Error("Stats not updated ?")
	}
}

func TestReadyNormal(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["servers"] = "1"
	m["hosts"] = "1"
	m["Ready"] = "0"
	stat := NewStats(m)
	if stat.Ready != 0 {
		t.Fatal("Stats should start with ready==0")
	}
	// First set up monitor listening
	mon := NewMonitor(stat)
	go mon.Listen()
	time.Sleep(100 * time.Millisecond)
	host := "localhost:" + strconv.Itoa(DefaultSinkPort)
	if stat.Ready != 0 {
		t.Fatal("Stats should have ready==0 after start of Monitor")
	}

	s, err := GetReady(host)
	if err != nil {
		t.Fatal("Couldn't get number of peers:", err)
	}
	if s.Ready != 0 {
		t.Fatal("Stats.Ready != 0")
	}

	err = Ready(host)
	if err != nil {
		t.Errorf("Error starting monitor: %s", err)
		return
	}

	s, err = GetReady(host)
	if err != nil {
		t.Fatal("Couldn't get number of peers:", err)
	}
	if s.Ready != 1 {
		t.Fatal("Stats.Ready != 1")
	}

	EndAndCleanup()
}

func TestKeyOrder(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["servers"] = "1"
	m["hosts"] = "1"
	m["bf"] = "2"
	m["rounds"] = "3"

	for i := 0; i < 20; i++ {
		// First set up monitor listening
		stat := NewStats(m)
		NewMonitor(stat)
		time.Sleep(100 * time.Millisecond)
		b := bytes.NewBuffer(make([]byte, 1024))
		stat.WriteHeader(b)
		dbg.Lvl2("Order:", strings.TrimSpace(b.String()))
		if strings.Contains(b.String(), "rounds, bf") {
			t.Fatal("Order of fields is not correct")
		}
	}
}

package monitor

import (
	"encoding/json"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"net"
	"strings"
	"sync"
)

// This file handles the collection of measurements, aggregates them and
// write CSV file reports

// listen is the address where to listen for the monitor. The endpoint can be a
// monitor.Proxy or a direct connection with measure.go
var listen string

// mutex is used to update the global stats from many connections
var mutex *sync.Mutex

func init() {
	mutex = &sync.Mutex{}
}

// Monitor will start listening for incoming connections on this address
// It needs the stats struct pointer to update when measures come
// This is a BLOCKING operation.
// Return an error if something went wrong during the connection setup
func Monitor(addr string, stats *Stats) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Error while monitor is binding address : %v", err)
	}
	var wg sync.WaitGroup
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				dbg.Lvl1("Error while monitor accept connection : ", err)
				continue
			}
			wg.Add(1)
			go handleConnection(conn, &wg, stats)
		}
	}()

	// Wait
	wg.Wait()

	return nil
}

// handleConnection will decode the data received and aggregates it into its
// stats
func handleConnection(conn net.Conn, wg *sync.WaitGroup, stats *Stats) {
	dec := json.NewDecoder(conn)
	var m Measure
	nerr := 0
	for {
		if err := dec.Decode(&m); err != nil {
			dbg.Lvl1("Error monitor decoding from ", conn.RemoteAddr().String(), " : ", err)
			nerr += 1
			if nerr > 1 {
				dbg.Lvl1("Monitor : too many errors from ", conn.RemoteAddr().String(), " : Abort.")
				break
			}
		}

		// Special case where the measurement is indicating a FINISHED step
		if strings.ToLower(m.Name) == "end" {
			break
		}
		updateMeasures(stats, m)
		m = Measure{}
	}
	// finished
	wg.Done()
}

// updateMeasures will add that specific measure to the global stats
// in a concurrently safe manner
func updateMeasures(stats *Stats, m Measure) {
	mutex.Lock()
	// updating
	stats.Update(m)
	mutex.Unlock()
}

package monitor

import (
	"encoding/json"
	"flag"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io"
	"net"
	"strings"
	"sync"
)

// This file handles the collection of measurements, aggregates them and
// write CSV file reports

// How many measures do I discard before aggregating the statistics
var Discard = 1

// listen is the address where to listen for the monitor. The endpoint can be a
// monitor.Proxy or a direct connection with measure.go
var Sink = "0.0.0.0"
var SinkPort = "10000"

// mutex is used to update the global stats from many connections
var mutex *sync.Mutex

var done = make(chan bool)

// Connections currently in use
var conns = make([]net.Conn, 0)

func init() {
	mutex = &sync.Mutex{}
	flag.IntVar(&Discard, "discard", 1, "How many measures do I discard before aggregating statistics (per-connections)")
}

// Monitor will start listening for incoming connections on this address
// It needs the stats struct pointer to update when measures come
// Return an error if something went wrong during the connection setup
func Monitor(stats *Stats) error {
	ln, err := net.Listen("tcp", Sink+":"+SinkPort)
	if err != nil {
		return fmt.Errorf("Error while monitor is binding address : %v", err)
	}
	dbg.Lvl2("Monitor listening for stats on ", Sink, ":", SinkPort)

	ch := make(chan net.Conn)
	var nconn int
	var finished bool = false
	go func() {
		for {
			if finished {
				break
			}
			conn, err := ln.Accept()
			if err != nil {
				operr, ok := err.(*net.OpError)
				// We cant accept anymore we closed the listener
				if ok && operr.Op == "accept" {
					break
				}
				dbg.Lvl2("Error while monitor accept connection : ", operr)
				continue
			}
			dbg.Lvl3("Monitor : new connection from ", conn.RemoteAddr().String())
			ch <- conn
		}
	}()
	for !finished {
		select {
		case c := <-ch:
			// TODO : maybe change to a more statefull approache with struct for each
			// connections...
			conns = append(conns, c)
			nconn += 1
			go handleConnection(c, stats)
		case <-done:
			nconn -= 1
			if nconn == 0 {
				ln.Close()
				finished = true
				break
			}
		}
	}
	dbg.Lvl2("Monitor finished waiting !")
	conns = make([]net.Conn, 0)
	return nil
}

// StopMonitor will close every connections it has
// And will stop updating the stats
func Stop() {
	dbg.Lvl2("Monitor Stop")
	for _, c := range conns {
		c.Close()
		done <- true
	}

}

// handleConnection will decode the data received and aggregates it into its
// stats
func handleConnection(conn net.Conn, stats *Stats) {
	dec := json.NewDecoder(conn)
	var m Measure
	nerr := 0
	nmes := 0
	for {
		if err := dec.Decode(&m); err != nil {
			// if end of connection
			if err == io.EOF {
				break
			}
			// otherwise log it
			dbg.Lvl2("Error monitor decoding from ", conn.RemoteAddr().String(), " : ", err)
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
		dbg.Lvl4("Monitor : received a Measure from ", conn.RemoteAddr().String(), " : ", m)
		if nmes > Discard {
			updateMeasures(stats, m)
		} else {
			dbg.Lvl4("Monitor: discarding measure")
			nmes += 1
		}
		m = Measure{}
	}
	// finished
	conn.Close()
	done <- true
}

// updateMeasures will add that specific measure to the global stats
// in a concurrently safe manner
func updateMeasures(stats *Stats, m Measure) {
	mutex.Lock()
	// updating
	stats.Update(m)
	mutex.Unlock()
}

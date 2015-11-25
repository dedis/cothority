// Monitor package handle the logging, collection and computation of
// statisticals data. Every application can send some Measure (for the moment,
// we mostly measure the CPU time but it can be applied later for any kind of
// measures). The Monitor receives them and update a Stats struct. This Statss
// struct can hold many different kinds of Measurement (the measure of an
// specific action such as "round time" or "verify time" etc). Theses
// measurements contains Values which compute the actual min/max/dev/avg values.
// There exists the Proxy file so we can have a Proxy relaying Measure from
// clients to the Monitor listening. An starter feature is also the DataFilter
// which can apply somes filtering rules to the data before making any
// statistics about them.
package monitor

import (
	"encoding/json"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io"
	"net"
	"strings"
	"sync"
)

// This file handles the collection of measurements, aggregates them and
// write CSV file reports

// listen is the address where to listen for the monitor. The endpoint can be a
// monitor.Proxy or a direct connection with measure.go
var Sink = "0.0.0.0"
var SinkPort = "10003"

// Monitor struct is used to collect measures and make the statistics about
// them. It takes a stats object so it update that in a concurrent-safe manner
// for each new measure it receives.
type Monitor struct {
	listener net.Listener

	// Current conections
	conns map[string]monitorConnection
	// and the mutex to play with it
	mutexConn sync.Mutex

	// Current stats
	stats *Stats
	// and the mutex to play with it
	mutexStats sync.Mutex

	// channel to give new measures
	measures chan Measure

	// channel to notify the end of a connection
	// send the name of the connection when finishd
	done chan string
}

// NewMonitor returns a new monitor given the stats
func NewMonitor(stats *Stats) Monitor {
	return Monitor{
		conns:      make(map[string]monitorConnection),
		stats:      stats,
		mutexStats: sync.Mutex{},
		measures:   make(chan Measure),
		done:       make(chan string),
	}
}

// Monitor will start listening for incoming connections on this address
// It needs the stats struct pointer to update when measures come
// Return an error if something went wrong during the connection setup
func (m *Monitor) Listen() error {
	ln, err := net.Listen("tcp", Sink+":"+SinkPort)
	if err != nil {
		return fmt.Errorf("Error while monitor is binding address : %v", err)
	}
	m.listener = ln
	dbg.Lvl2("Monitor listening for stats on ", Sink, ":", SinkPort)
	finished := false
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
			m.mutexConn.Lock()
			mc := monitorConnection{
				conn:  conn,
				done:  m.done,
				stats: m.measures,
			}
			go mc.handleConnection()
			m.conns[conn.RemoteAddr().String()] = mc
			m.mutexConn.Unlock()
		}
	}()
	for !finished {
		select {
		// new stats
		case measure := <-m.measures:
			m.update(measure)
			// end of a peer conn
		case peer := <-m.done:
			m.mutexConn.Lock()
			delete(m.conns, peer)
			// end of monitoring,
			if len(m.conns) == 0 {
				m.listener.Close()
				finished = true
				m.mutexConn.Unlock()
				break
			}
		}
	}
	dbg.Lvl2("Monitor finished waiting !")
	m.conns = make(map[string]monitorConnection)
	return nil
}

// StopMonitor will close every connections it has
// And will stop updating the stats
func (m *Monitor) Stop() {
	dbg.Lvl2("Monitor Stop")
	m.listener.Close()
	m.mutexConn.Lock()
	for _, c := range m.conns {
		c.Stop()
	}
	m.mutexConn.Unlock()

}

// monitorConnection represents a statefull connection from a proxy or a client
// to the monitor
type monitorConnection struct {
	conn net.Conn
	// For telling when to stop AND when the connection is closed
	done chan string
	// Giving the stats back to monitor
	stats chan Measure
}

// Stop will close this connection and notify the monitor associated
func (mc *monitorConnection) Stop() {
	str := mc.conn.RemoteAddr().String()
	mc.conn.Close()
	mc.done <- str
}

// handleConnection will decode the data received and aggregates it into its
// stats
func (mc *monitorConnection) handleConnection() {
	dec := json.NewDecoder(mc.conn)
	var m Measure
	nerr := 0
	for {
		if err := dec.Decode(&m); err != nil {
			// if end of connection
			if err == io.EOF {
				break
			}
			// otherwise log it
			dbg.Lvl2("Error monitor decoding from ", mc.conn.RemoteAddr().String(), " : ", err)
			nerr += 1
			if nerr > 1 {
				dbg.Lvl2("Monitor : too many errors from ", mc.conn.RemoteAddr().String(), " : Abort.")
				break
			}
		}

		// Special case where the measurement is indicating a FINISHED step
		if strings.ToLower(m.Name) == "end" {
			break
		}
		dbg.Lvl4("Monitor : received a Measure from ", mc.conn.RemoteAddr().String(), " : ", m)
		mc.stats <- m
		m = Measure{}
	}
	// finished
	mc.Stop()
}

// updateMeasures will add that specific measure to the global stats
// in a concurrently safe manner
func (m *Monitor) update(meas Measure) {
	m.mutexStats.Lock()
	// updating
	m.stats.Update(meas)
	//dbg.Print("Stats = ", m.stats)
	m.mutexStats.Unlock()
}

// Stats returns the updated stats in a concurrent-safe manner
func (m *Monitor) Stats() *Stats {
	m.mutexStats.Lock()
	s := m.stats
	m.mutexStats.Unlock()
	return s
}

/*
 * Time-measurement functions.
 *
 * Usage:
 * ```measure := monitor.NewMeasure()```
 * ```// Do some calculations```
 * ```measure.MeasureWall("CPU on calculations")```
 */

package monitor

import (
	"encoding/json"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"net"
	"syscall"
	"time"
)

// Sink is the server address where all the measures will be received and
// analyzed
var sink string

// We use json to encode our struct
var encoder *json.Encoder
var connection net.Conn

// enabled notify if we want to use the monitor or not. If we call Disable(),
// the code stay the same but every call to Measure() won't just do a thing.
var enabled bool = true

// ConnectSink will connect to the endpoint given and initialize our json
// encoder. It can be a proxy address or directly a monitoring process address.
// Return an error if it could not connect to the endpoint
func ConnectSink(addr string) error {
	dbg.Lvl2("ConnectSink attempt with ", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	dbg.Lvl2("Connected to sink : ", addr)
	sink = addr
	connection = conn
	encoder = json.NewEncoder(conn)
	return nil
}

// Send will send the given struct to the network
func send(v interface{}) {
	if encoder == nil {
		panic(fmt.Errorf("Monitor's sink connection not initalized. Can not send any measures"))
	}
	if !enabled {
		return
	}
	if err := encoder.Encode(v); err != nil {
		panic(fmt.Errorf("Error sending to sink : %v", err))
	}
}

// Disable / Enable the monitoring library
func Disable() {
	dbg.Lvl3("Monitor Measure disabled")
	enabled = false
}
func Enable() {
	dbg.Lvl3("Monitor Measure enabled")
	enabled = true
}

// Measure holds the different values taht can be computed for a measure
// It is what the client sends to the monitor.
type Measure struct {
	Name        string
	WallTime    float64
	CPUTimeUser float64
	CPUTimeSys  float64
	// Since we send absolute timing values, we need to store our reference also
	lastWallTime time.Time
	allowUpdate  bool
}

// Creates a new measure-struct
// Automatically update the time when we call Measure
func NewMeasure(name string) *Measure {
	m := &Measure{Name: name}
	m.Update()
	m.UpdatePref(true)
	return m
}

func (m *Measure) Measure() {
	// Wall time measurement
	m.WallTime = float64(time.Since(m.lastWallTime)) / 1.0e9
	// CPU time measurement
	m.CPUTimeSys, m.CPUTimeUser = getDiffRTime(m.CPUTimeSys, m.CPUTimeUser)
	// send the data
	send(m)
	m.Update()

}

// Whether the measurement should be updated automatically
// after each MeasureCPU and MeasureWall. If called with
// true, updates also the clock
func (m *Measure) UpdatePref(allow bool) {
	m.allowUpdate = allow
	m.Update()
}

// Sets 'now' as the start-time
func (m *Measure) Update() {
	if m.allowUpdate {
		m.CPUTimeSys, m.CPUTimeUser = GetRTime()
		m.lastWallTime = time.Now()
	}
}

// Prints a message to end the logging
func End() {
	send(Measure{Name: "end"})
	connection.Close()
}

// Convert microseconds to seconds
func iiToF(sec int64, usec int64) float64 {
	return float64(sec) + float64(usec)/1000000.0
}

// Gets the sytem and the user time so far
func GetRTime() (tSys, tUsr float64) {
	rusage := &syscall.Rusage{}
	syscall.Getrusage(syscall.RUSAGE_SELF, rusage)
	s, u := rusage.Stime, rusage.Utime
	return iiToF(int64(s.Sec), int64(s.Usec)), iiToF(int64(u.Sec), int64(u.Usec))
}

// Returns the difference to the given system- and user-time
func getDiffRTime(tSys, tUsr float64) (tDiffSys, tDiffUsr float64) {
	nowSys, nowUsr := GetRTime()
	return nowSys - tSys, nowUsr - tUsr
}

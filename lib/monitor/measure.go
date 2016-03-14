package monitor

import (
	"encoding/json"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

// Sink is the server address where all measures are transmitted to for
// further analysis.
var sink string

// Structs are encoded through a json encoder.
var encoder *json.Encoder
var connection net.Conn

// Keeps track if a measure is enabled (true) or not (false). If disabled,
// measures are not sent to the monitor. Use EnableMeasure(bool) to toggle
// this variable.
var enabled = true

// Generic interface for measurements
// Usage:
// ```measure := monitor.SingleMeasure("bandwidth")```
// or
// ```measure := monitor.NewTimeMeasure("round")
// ```measure.Record()```
//
type Measure interface {
	// Record must be called when you want to send the value
	// over the monitor listening.
	// Implementation of this interface must RESET the value to `0` at the end
	// of Record(). `0` means the initial value / meaning this measure had when
	// created.
	// Example: TimeMeasure.Record() will reset the time to `time.Now()`
	//          CounterIOMeasure.Record() will  reset the counter of the bytes
	//          read / written to 0.
	//          etc
	Record()
}

// SingleMeasure is a pair name - value we want to send
type SingleMeasure struct {
	Name  string
	Value float64
}

// TimeMeasure represents a measure regarding time: It includes the wallclock
// time, the cpu time + the user time.
type TimeMeasure struct {
	Wall *SingleMeasure
	Cpu  *SingleMeasure
	User *SingleMeasure
	// non exported fields
	// name of the time measure (basename)
	name string
	// last time
	lastWallTime time.Time
}

// ConnectSink connects to the given endpoint and initialises a json
// encoder. It can be the address of a proxy or a monitoring process.
// Returns an error if it could not connect to the endpoint.
func ConnectSink(addr string) error {
	if encoder != nil {
		return nil
	}
	dbg.Lvl3("Connecting to:", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	dbg.Lvl3("Connected to sink:", addr)
	sink = addr
	connection = conn
	encoder = json.NewEncoder(conn)
	return nil
}

func StopSink() {
	if err := connection.Close(); err != nil {
		// at least tell that we could not close the connection:
		dbg.Error("Could not close connecttion:", err)
	}
	encoder = nil
}

// NewSingleMeasure returns a new measure freshly generated
func NewSingleMeasure(name string, value float64) *SingleMeasure {
	return &SingleMeasure{
		Name:  name,
		Value: value,
	}
}

// Record sends the value to the monitor. Reset the value to 0.
func (sm *SingleMeasure) Record() {
	if err := send(sm); err != nil {
		dbg.Error("Error sending SingleMeasure", sm.Name, " to monitor:", err)
	}
	sm.Value = 0
}

// NewTimeMeasure return *TimeMeasure
func NewTimeMeasure(name string) *TimeMeasure {
	tm := &TimeMeasure{name: name}
	tm.reset()
	return tm
}

func (tm *TimeMeasure) Record() {
	// Wall time measurement
	tm.Wall = NewSingleMeasure(tm.name+"_wall", float64(time.Since(tm.lastWallTime))/1.0e9)
	// CPU time measurement
	tm.Cpu.Value, tm.User.Value = getDiffRTime(tm.Cpu.Value, tm.User.Value)
	// send data
	tm.Wall.Record()
	tm.Cpu.Record()
	tm.User.Record()
	// reset timers
	tm.reset()

}

// reset reset the time fields of this time measure
func (tm *TimeMeasure) reset() {
	cpuTimeSys, cpuTimeUser := getRTime()
	tm.Cpu = NewSingleMeasure(tm.name+"_system", cpuTimeSys)
	tm.User = NewSingleMeasure(tm.name+"_user", cpuTimeUser)
	tm.lastWallTime = time.Now()
}

// CounterIO is an interface that can be used to count how many bytes does an
// object have written and how many bytes does it have read. For example it is
// implemented by cothority/network/ Conn  + Host to know how many bytes a
// connection / Host has written /read
type CounterIO interface {
	Read() uint64
	Written() uint64
}

// CounterIOMeasure is a struct that takes a CounterIO and can send the
// measurements to the monitor. Each time Record() is called, the measurements
// are put back to 0 (while the CounterIO still sends increased bytes number).
type CounterIOMeasure struct {
	name        string
	counter     CounterIO
	baseWritten uint64
	baseRead    uint64
}

// NewCounterIOMeasure returns an CounterIOMeasure fresh. The base value are set
// to the current value of counter.Read() and counter.Written()
func NewCounterIOMeasure(name string, counter CounterIO) *CounterIOMeasure {
	return &CounterIOMeasure{
		name:        name,
		counter:     counter,
		baseWritten: counter.Written(),
		baseRead:    counter.Read(),
	}
}

// Record send the actual number of bytes read and written (**name**_written &
// **name**_read) and reset the counters.
func (cm *CounterIOMeasure) Record() {
	// creates the read measure
	bRead := cm.counter.Read()
	// TODO Later on, we might want to do a check on the conversion between
	// uint64 -> float64, as the MAX values are not the same.
	read := NewSingleMeasure(cm.name+"_read", float64(bRead-cm.baseRead))
	// creates the  written measure
	bWritten := cm.counter.Written()
	written := NewSingleMeasure(cm.name+"_written", float64(bWritten-cm.baseWritten))

	// send them both
	read.Record()
	written.Record()

	// reset counters
	cm.baseRead = bRead
	cm.baseWritten = bWritten
}

// Send transmits the given struct over the network.
func send(v interface{}) error {
	if encoder == nil {
		return fmt.Errorf("Monitor's sink connection not initalized. Can not send any measures")
	}
	if !enabled {
		return nil
	}
	// For a large number of clients (˜10'000), the connection phase
	// can take some time. This is a linear backoff to enable connection
	// even when there are a lot of request:
	for wait := 500; wait < 1000; wait += 100 {
		if err := encoder.Encode(v); err == nil {
			return nil
		} else {
			dbg.Lvl1("Couldn't send to monitor-sink:", err)
			time.Sleep(time.Duration(wait) * time.Millisecond)
		}
	}
	return fmt.Errorf("No contact to monitor-sink possible!")
}

// Prints a message to end the logging.
func EndAndCleanup() {
	send(NewSingleMeasure("end", 0))
	StopSink()
}

// Converts microseconds to seconds.
func iiToF(sec int64, usec int64) float64 {
	return float64(sec) + float64(usec)/1000000.0
}

// Returns the sytem and the user time so far.
func getRTime() (tSys, tUsr float64) {
	rusage := &syscall.Rusage{}
	syscall.Getrusage(syscall.RUSAGE_SELF, rusage)
	s, u := rusage.Stime, rusage.Utime
	return iiToF(int64(s.Sec), int64(s.Usec)), iiToF(int64(u.Sec), int64(u.Usec))
}

// Returns the difference of the given system- and user-time.
func getDiffRTime(tSys, tUsr float64) (tDiffSys, tDiffUsr float64) {
	nowSys, nowUsr := getRTime()
	return nowSys - tSys, nowUsr - tUsr
}

// Enables / Disables a measure.
func EnableMeasure(b bool) {
	if b {
		dbg.Lvl3("Monitor: Measure enabled")
	} else {
		dbg.Lvl3("Monitor: Measure disabled")
	}
	enabled = b
}

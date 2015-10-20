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
	"time"
	"syscall"
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/logutils"
)

type Measure struct {
	WallTime    time.Time
	CPUTimeUser float64
	CPUTimeSys  float64
	allowUpdate bool
}

// Creates a new measure-struct
func NewMeasure() *Measure {
	m := &Measure{}
	m.Update()
	return m
}

// Prints the wall-clock time used for that measurement,
// also updates the clock
func (m *Measure)MeasureWall(message string, d ...float64) {
	div := 1.0
	if len(d) > 0 {
		div = d[0]
	}
	logWF(message, float64(time.Since(m.WallTime)) / 1.0e9 / div)
	m.Update()
}

// Prints the CPU-clock time used for that measurement,
// also updates the clock
func (m *Measure)MeasureCPU(message string, d ...float64) {
	div := 1.0
	if len(d) > 0 {
		div = d[0]
	}
	sys, usr := getDiffRTime(m.CPUTimeSys, m.CPUTimeUser)
	logWF(message, (sys + usr) / div)
	m.Update()
}

// Prints the wall-clock and the CPU-time used for that measurement,
// also updates the clock
func (m *Measure)MeasureCPUWall(messageCPU, messageWall string, d ...float64) {
	au := m.allowUpdate
	m.UpdatePref(false)
	m.MeasureCPU(messageCPU)
	m.MeasureWall(messageWall)
	m.UpdatePref(au)
}

// Whether the measurement should be updated automatically
// after each MeasureCPU and MeasureWall. If called with
// true, updates also the clock
func (m *Measure)UpdatePref(allow bool) {
	m.allowUpdate = allow
	m.Update()
}

// Sets 'now' as the start-time
func (m *Measure)Update() {
	if m.allowUpdate {
		m.CPUTimeSys, m.CPUTimeUser = GetRTime()
		m.WallTime = time.Now()
	}
}

// Prints a message to end the logging
func LogEnd() {
	logWF("end", 0.0)
}

// Convert microseconds to seconds
func iiToF(sec int64, usec int64) float64 {
	return float64(sec) + float64(usec) / 1000000.0
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

// Writes a message to the logger that will be caught by the
// main system
func logWF(message string, time float64) {
	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": message,
		"time": time}).Info("")
}


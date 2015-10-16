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
}

func NewMeasure() *Measure {
	m := &Measure{}
	m.Update()
	return m
}

func (m *Measure)Update(){
	m.CPUTimeSys, m.CPUTimeUser = GetRTime()
	m.WallTime = time.Now()
}

func (m *Measure)MeasureWall(message string, d ...int) {
	div := 1
	if len(d) > 0{
		div = d[0]
	}
	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": message,
		"time": float64(time.Since(m.WallTime)) / 1.e9 / div}).Info("")
}

func (m *Measure)MeasureCPU(message string, d ...int) {
	div := 1
	if len(d) > 0{
		div = d[0]
	}
	sys, usr := GetDiffRTime(m.CPUTimeSys, m.CPUTimeUser)
	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": message,
		"time": (sys + usr) / div}).Info("")
}

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
func GetDiffRTime(tSys, tUsr float64) (tDiffSys, tDiffUsr float64) {
	nowSys, nowUsr := GetRTime()
	return nowSys - tSys, nowUsr - tUsr
}

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

func (m *Measure)MeasureWall(message string, d ...float64) {
	div := 1.0
	if len(d) > 0{
		div = d[0]
	}
	logWF(message, float64(time.Since(m.WallTime)) / 1.0e9 / div)
	m.Update()
}

func (m *Measure)MeasureCPU(message string, d ...float64) {
	div := 1.0
	if len(d) > 0{
		div = d[0]
	}
	sys, usr := getDiffRTime(m.CPUTimeSys, m.CPUTimeUser)
	logWF(message, (sys + usr) / div)
	m.Update()
}

func (m *Measure)Update(){
	m.CPUTimeSys, m.CPUTimeUser = GetRTime()
	m.WallTime = time.Now()
}

func LogEnd(){
	logWF("end", 0.0)
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
func getDiffRTime(tSys, tUsr float64) (tDiffSys, tDiffUsr float64) {
	nowSys, nowUsr := GetRTime()
	return nowSys - tSys, nowUsr - tUsr
}

func logWF(message string, time float64){
	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": message,
		"time": time}).Info("")
}


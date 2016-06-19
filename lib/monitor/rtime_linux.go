// +build linux darwin

package monitor

import (
	"syscall"
	"github.com/dedis/cothority/lib/dbg"
)

// Returns the system and the user CPU time used by the current process so far.
func getRTime() (tSys, tUsr float64) {
	rusage := &syscall.Rusage{}
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, rusage); err != nil {
		dbg.Error("Couldn't get rusage time:", err)
	}
	s, u := rusage.Stime, rusage.Utime
	return iiToF(int64(s.Sec), int64(s.Usec)), iiToF(int64(u.Sec), int64(u.Usec))
}

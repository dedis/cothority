package lib

import (
	"strings"
	"time"

	"github.com/dedis/onet/log"
)

// Speed can be used to measure timing in tests
type Speed struct {
	Start time.Time
}

// NewSpeed returns a new speed
func NewSpeed() *Speed {
	return &Speed{Start: time.Now()}
}

// Done prints the time spent
func (s *Speed) Done() {
	method := strings.Split(strings.Split(log.Stack(), "\n")[7], "(")[0]
	timeStr := time.Since(s.Start).Seconds()
	log.Printf("Time: %03ds for %s - %.3f", int(timeStr), method, timeStr)
}

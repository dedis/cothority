package network

import "github.com/dedis/cothority/monitor"

// Listener is analoguous to net.Listener. It listens on a port, and will call a
// handler each time a new incoming connection event occurs. The type of the
// connection that Listener accepts MUST be of only one type,i.e. if
type Listener struct {
	Listen (func(Conn))
}

// Host is the basic interface to represent a Host of any kind
// Host can open new Conn(ections) and Listen for any incoming Conn(...)
type Host interface {
	Open(name string) (Conn, error)
	Listen(addr string, fn func(Conn)) error // the srv processing function
	Close() error
	monitor.CounterIO
}

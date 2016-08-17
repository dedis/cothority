package network

import (
	"github.com/dedis/cothority/monitor"
	"golang.org/x/net/context"
)

// Conn is the basic interface to represent any communication mean
// between two host.
type Conn interface {
	// Gives the address of the remote endpoint
	Remote() string
	// Returns the local address and port
	Local() string
	// Send a message through the connection.
	// obj should be a POINTER to the actual struct to send, or an interface.
	// It should not be a Golang type.
	Send(ctx context.Context, obj Body) error
	// Receive any message through the connection.
	Receive(ctx context.Context) (Packet, error)
	// Close will close the connection. Any subsequent call to Send / Receive
	// have undefined behavior.
	Close() error
	// XXX Can we remove that ?
	monitor.CounterIO
}

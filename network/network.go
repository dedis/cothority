package network

import (
	"context"

	"github.com/dedis/cothority/monitor"
)

// Conn is the basic interface to represent any communication mean
// between two host.
type Conn interface {
	// Send a message through the connection.
	// obj should be a POINTER to the actual struct to send, or an interface.
	// It should not be a Golang type.
	Send(ctx context.Context, obj Body) error
	// Receive any message through the connection.
	Receive(ctx context.Context) (Packet, error)
	// Close will close the connection. Any subsequent call to Send / Receive
	// have undefined behavior.
	Close() error

	// Type returns the type of this connection
	Type() ConnType
	// Gives the address of the remote endpoint
	Remote() Address
	// Returns the local address and port
	Local() Address
	// reconnect is used when sending a message to a Conn, we might want to try
	// to reconnect directly if an error occured to send the message again.
	//reconnect() error
	// XXX Can we remove that ?
	monitor.CounterIO
}

// Listener is responsible for listening for incoming Conn on a particular
// address.It can only accept one type of incoming Conn.
type Listener interface {
	// Listen will start listening for incoming connections
	// Each time there is an incoming Conn, it will call the given
	// function in a go routine with the incoming Conn as parameter.
	// The call is BLOCKING.
	Listen(func(Conn)) error
	// Stop will stop the listening. Implementations must take care of making
	// Stop() a blocking call. Stop() should return when the Listener really
	// has stopped listening,i.e. the call to Listen has returned.
	Stop() error

	// what is the address this listener is listening to + what type of
	// connection does it accept (address.ConnType())
	Address() Address
}

// Host is an interface that can Listen for a specific type of Conn and can
// Connect to specific types of Conn. It used by the Router so the router can
// manage connections all being oblivious to which type of connections.
type Host interface {
	Listener

	Connect(addr Address) (Conn, error)
}

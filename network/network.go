package network

import (
	"golang.org/x/net/context"

	"github.com/dedis/cothority/monitor"
)

// Conn is the basic interface to represent any communication mean
// between two host.
type Conn interface {
	// Send a message through the connection.
	// obj should be a POINTER to the actual struct to send, or an interface.
	// It should not be a Golang type.
	Send(ctx context.Context, obj Body) error
	// Receive any message through the connection. It is a blocking call that
	// returns either when a message arrived or when Close() has been called, or
	// when a network error occured.
	Receive(ctx context.Context) (Packet, error)
	// Close will close the connection. Implementations must take care that
	// Close() makes Receive() returns with an error, and any subsequent Send()
	// will return with an error. Calling Close() on a closed Conn will return
	// ErrClosed.
	Close() error

	// Type returns the type of this connection
	Type() ConnType
	// Gives the address of the remote endpoint
	Remote() Address
	// Returns the local address and port
	Local() Address
	// XXX Can we remove that ?
	monitor.CounterIO
}

// Listener is responsible for listening for incoming Conn on a particular
// address.It can only accept one type of incoming Conn.
type Listener interface {
	// Listen will start listening for incoming connections
	// Each time there is an incoming Conn, it will call the given
	// function in a go routine with the incoming Conn as parameter.
	// The call is BLOCKING. If this listener is already Listening, Listen
	// should return an error.
	Listen(func(Conn)) error
	// Stop will stop the listening. Implementations must take care of making
	// Stop() a blocking call. Stop() should return when the Listener really
	// has stopped listening,i.e. the call to Listen has returned. Calling twice
	// Stop() should return an error ErrClosed on the second call.
	Stop() error

	// what is the address this listener is listening to + what type of
	// connection does it accept (address.ConnType())
	Address() Address

	// Returns whether this listener is actually listening or not. Sadly this
	// function is mainly useful for tests where we need to make sure the
	// listening routine is started.
	Listening() bool
}

// Host is an interface that can Listen for a specific type of Conn and can
// Connect to specific types of Conn. It used by the Router so the router can
// manage connections all being oblivious to which type of connections.
type Host interface {
	Listener

	Connect(addr Address) (Conn, error)
}

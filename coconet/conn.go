package coconet

import (
	"github.com/dedis/crypto/abstract"
)

// Conn is an abstract bidirectonal connection.
// It abstracts away the network layer as well as the data-format for communication.
type Conn interface {
	// Name returns the name of the "to" end of the connectio.
	Name() string

	// PubKey returns the public key associated with the peer.
	PubKey() abstract.Point
	SetPubKey(abstract.Point)

	// Put puts data to the connection, calling the MarshalBinary method as needed.
	Put(data BinaryMarshaler) error
	// Get gets data from the connection, calling the UnmarshalBinary method as needed.
	// It blocks until it successfully receives data or there was a network error.
	// It returns io.EOF if the channel has been closed.
	Get(data BinaryUnmarshaler) error

	// Connect establishes the connection. Before using the Put and Get
	// methods of a Conn, Connect must first be called.
	Connect() error

	// Close closes the connection. Any blocked Put or Get operations will
	// be unblocked and return errors.
	Close()

	// Indicates whether the connection has been closed
	Closed() bool
}

// Taken from: http://golang.org/pkg/encoding/#BinaryMarshaler
// All messages passing through our conn must implement their own  BinaryMarshaler
type BinaryMarshaler interface {
	MarshalBinary() (data []byte, err error)
}

// Taken from: http://golang.org/pkg/encoding/#BinaryMarshaler
// All messages passing through our conn must implement their own BinaryUnmarshaler
type BinaryUnmarshaler interface {
	UnmarshalBinary(data []byte) error
}

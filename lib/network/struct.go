package network

import (
	"errors"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"net"
	"time"
)

// How many times should we try to connect
const maxRetry = 10
const waitRetry = 1 * time.Second
const timeOut = 5 * time.Second

// The various errors you can have
// XXX not working as expected, often falls on errunknown
var ErrClosed = errors.New("Connection Closed")
var ErrEOF = errors.New("EOF")
var ErrCanceled = errors.New("Operation Canceled")
var ErrTemp = errors.New("Temporary Error")
var ErrTimeout = errors.New("Timeout Error")
var ErrUnknown = errors.New("Unknown Error")

// Host is the basic interface to represent a Host of any kind
// Host can open new Conn(ections) and Listen for any incoming Conn(...)
type Host interface {
	Open(name string) (Conn, error)
	Listen(addr string, fn func(Conn)) error // the srv processing function
	Close() error
}

// Conn is the basic interface to represent any communication mean
// between two host. It is closely related to the underlying type of Host
// since a TcpHost will generate only TcpConn
type Conn interface {
	// Gives the address of the remote endpoint
	Remote() string
	// Send a message through the connection. Always pass a pointer !
	Send(ctx context.Context, obj NetworkMessage) error
	// Receive any message through the connection.
	Receive(ctx context.Context) (ApplicationMessage, error)
	Close() error
}

// TcpHost is the underlying implementation of
// Host using Tcp as a communication channel
type TcpHost struct {
	// A list of connection maintained by this host
	peers map[string]Conn
	// its listeners
	listener net.Listener
	// the close channel used to indicate to the listener we want to quit
	quit chan bool
	// indicates wether this host is closed already or not
	closed bool
	// a list of constructors for en/decoding
	constructors protobuf.Constructors
}

// TcpConn is the underlying implementation of
// Conn using Tcp
type TcpConn struct {
	// The name of the endpoint we are connected to.
	Endpoint string

	// The connection used
	Conn net.Conn

	// closed indicator
	closed bool
	// A pointer to the associated host (just-in-case)
	host *TcpHost
}

// SecureHost is the analog of Host but with secure communication
// It is tied to an entity can only open connection with entities
type SecureHost interface {
	Close() error
	Listen(func(SecureConn)) error
	Open(*Entity) (SecureConn, error)
}

// SecureConn is the analog of Conn but for secure communication
type SecureConn interface {
	Conn
	Entity() *Entity
}

// SecureTcpHost is a TcpHost but with the additional property that it handles
// Entity. You
type SecureTcpHost struct {
	*TcpHost
	// Entity of this host
	entity *Entity
	// Private key tied to this entity
	private abstract.Secret
	// mapping from the entity to the names used in TcpHost
	// In TcpHost the names then maps to the actual connection
	EntityToAddr map[uuid.UUID]string
	// workingaddress is a private field used mostly for testing
	// so we know which address this host is listening on
	workingAddress string
}

type SecureTcpConn struct {
	*TcpConn
	*SecureTcpHost
	entity *Entity
}

// EntityToml is the struct that can be marshalled into a toml file
type EntityToml struct {
	Public    string
	Addresses []string
}

// Entity is used to represent a Conode in the whole internet.
// It's based on a public key, and there can be one or more addresses to contact it.
type Entity struct {
	// This is the public key of that Entity
	Public abstract.Point
	// The UUID corresponding to that public key
	Id uuid.UUID
	// A slice of addresses of where that Id might be found
	Addresses []string
	// used to return the next available address
	iter int
}

// EntityType can be used to recognise an Entity-message
var EntityType = RegisterMessageType(Entity{})

// ApplicationMessage is the container for any NetworkMessage
type ApplicationMessage struct {
	// The Entity of the remote peer we are talking to.
	// Basically, this means that when you open a new connection to someone, and
	// / or listens to incoming connections, the network library will already
	// make some exchange between the two communicants so each knows the
	// Entity of the others.
	Entity *Entity
	// the origin of the message
	From string
	// What kind of msg do we have
	MsgType uuid.UUID
	// The underlying message
	Msg NetworkMessage
	// which constructors are used
	Constructors protobuf.Constructors
	// possible error during unmarshaling so that upper layer can know it
	err error
}

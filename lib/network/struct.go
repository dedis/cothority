package network

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// How many times should we try to connect
const maxRetry = 10
const waitRetry = 100 * time.Millisecond
const timeOut = 3000 * time.Second

// The various errors you can have
// XXX not working as expected, often falls on errunknown
var ErrClosed = errors.New("Connection Closed")
var ErrEOF = errors.New("EOF")
var ErrCanceled = errors.New("Operation Canceled")
var ErrTemp = errors.New("Temporary Error")
var ErrTimeout = errors.New("Timeout Error")
var ErrUnknown = errors.New("Unknown Error")

type Size uint32

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
	Send(ctx context.Context, obj ProtocolMessage) error
	// Receive any message through the connection.
	Receive(ctx context.Context) (NetworkMessage, error)
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
	// quitListener is a channel to indicate to the closing function that the
	// listener has actually really quit
	quitListener  chan bool
	listeningLock *sync.Mutex
	listening     bool
	// indicates wether this host is closed already or not
	closed     bool
	closedLock *sync.Mutex
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
// Entity.
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

// SecureTcpConn is a secured tcp connection using Entity as identity
type SecureTcpConn struct {
	*TcpConn
	*SecureTcpHost
	entity *Entity
}

// NetworkMessage is the container for any NetworkMessage
type NetworkMessage struct {
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
	Msg ProtocolMessage
	// which constructors are used
	Constructors protobuf.Constructors
	// possible error during unmarshaling so that upper layer can know it
	err error
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

func (e *Entity) String() string {
	return fmt.Sprintf("%v", e.Addresses)
}

// EntityType can be used to recognise an Entity-message
var EntityType = RegisterMessageType(Entity{})

// EntityToml is the struct that can be marshalled into a toml file
type EntityToml struct {
	Public    string
	Addresses []string
}

// NewEntity creates a new Entity based on a public key and with a slice
// of IP-addresses where to find that entity. The Id is based on a
// version5-UUID which can include a URL that is based on it's public key.
func NewEntity(public abstract.Point, addresses ...string) *Entity {
	url := UuidURL + "id/" + public.String()
	return &Entity{
		Public:    public,
		Addresses: addresses,
		Id:        uuid.NewV5(uuid.NamespaceURL, url),
	}
}

// First returns the first address available
func (e *Entity) First() string {
	if len(e.Addresses) > 0 {
		return e.Addresses[0]
	}
	return ""
}

// Next returns the next address like an iterator,
// starting at the beginning if nothing worked
func (e *Entity) Next() string {
	addr := e.Addresses[e.iter]
	e.iter = (e.iter + 1) % len(e.Addresses)
	return addr

}

// Equal tests on same public key
func (e *Entity) Equal(e2 *Entity) bool {
	return e.Public.Equal(e2.Public)
}

// Toml converts an Entity to a Toml-structure
func (e *Entity) Toml(suite abstract.Suite) *EntityToml {
	var buf bytes.Buffer
	cliutils.WritePub64(suite, &buf, e.Public)
	return &EntityToml{
		Addresses: e.Addresses,
		Public:    buf.String(),
	}
}

// Entity converts an EntityToml structure back to an Entity
func (e *EntityToml) Entity(suite abstract.Suite) *Entity {
	pub, _ := cliutils.ReadPub64(suite, strings.NewReader(e.Public))
	return &Entity{
		Public:    pub,
		Addresses: e.Addresses,
	}
}

// handleError produces the higher layer error depending on the type
// so user of the package can know what is the cause of the problem
func handleError(err error) error {

	if strings.Contains(err.Error(), "use of closed") {
		return ErrClosed
	} else if strings.Contains(err.Error(), "canceled") {
		return ErrCanceled
	} else if err == io.EOF || strings.Contains(err.Error(), "EOF") {
		return ErrEOF
	}

	netErr, ok := err.(net.Error)
	if !ok {
		return ErrUnknown
	}
	if netErr.Temporary() {
		return ErrTemp
	} else if netErr.Timeout() {
		return ErrTimeout
	}
	return ErrUnknown
}

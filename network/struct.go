package network

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
)

// MaxRetryConnect defines how many times we should try to connect.
const MaxRetryConnect = 10

// MaxIdentityExchange is the timeout for an identityExchange.
const MaxIdentityExchange = 5 * time.Second

// WaitRetry is the timeout on connection-setups.
const WaitRetry = 100 * time.Millisecond

// The various errors you can have
// XXX not working as expected, often falls on errunknown

// ErrClosed is when a connection has been closed.
var ErrClosed = errors.New("Connection Closed")

// ErrEOF is when the connection sends an EOF signal (mostly because it has
// been shut down).
var ErrEOF = errors.New("EOF")

// ErrCanceled means something went wrong in the sending or receiving part.
var ErrCanceled = errors.New("Operation Canceled")

// ErrTemp is a temporary error, recovery possible.
var ErrTemp = errors.New("Temporary Error")

// ErrTimeout is raised if the timeout has been reached.
var ErrTimeout = errors.New("Timeout Error")

// ErrUnknown is an unknown error.
var ErrUnknown = errors.New("Unknown Error")

// Size is a type to reprensent the size that is sent before every packet to
// correctly decode it.
type Size uint32

// Host represent a given connection and transmission protocol.
// Host can open new Conn(ections) and Listen for any incoming Conn(...).
// Depending on the host it is possible that it can connect with hosts of
// other types or not.
type Host interface {
	Open(name string) (Conn, error)
	Listen(addr string, fn func(Conn)) error // the srv processing function
	Close() error
	monitor.CounterIO
}

// Conn represents a communication between two hosts. It is closely related 
// to the underlying type of Host since a TcpHost will generate only TcpConn.
type Conn interface {
	// The address of the remote endpoint.
	Remote() string
	// The local address and port.
	Local() string
	// Sends a message through the connection. The obj always has to be
	// a pointer.
	Send(ctx context.Context, obj Body) error
	// Receives a message from the connection.
	Receive(ctx context.Context) (Packet, error)
	// Close shuts down the connection
	Close() error
	monitor.CounterIO
}

// TCPHost implements Host to offer a bare, unencrypted TCP communication.
type TCPHost struct {
	// listeningPort will channel the port used in the connection.
	listeningPort chan int
	// A list of connections maintained by this host.
	peers    map[string]Conn
	// Mutex on peers.
	peersMut sync.Mutex
	// The listener attached to this host.
	listener net.Listener
	// the close channel used to indicate to the listener we want to quit.
	quit chan bool
	// quitListener is a channel to indicate to the closing function that the
	// listener has actually really quit
	quitListener  chan bool
	listeningLock sync.Mutex
	listening     bool
	// indicates whether this host is closed already or not
	closed     bool
	closedLock sync.Mutex
	// a list of constructors for en/decoding
	constructors protobuf.Constructors
}

// TCPConn is the underlying implementation of
// Conn using Tcp
type TCPConn struct {
	// The name of the endpoint we are connected to.
	Endpoint string

	// The connection used
	conn net.Conn

	// closed indicator
	closed    bool
	closedMut sync.Mutex
	// A pointer to the associated host (just-in-case)
	host *TCPHost
	// So we only handle one receiving packet at a time
	receiveMutex sync.Mutex
	// So we only handle one sending packet at a time
	sendMutex sync.Mutex
	// bRx is the number of bytes received on this connection
	bRx     uint64
	bRxLock sync.Mutex
	// bTx in the number of bytes sent on this connection
	bTx     uint64
	bTxLock sync.Mutex
}

// SecureHost is the analog of Host but with secure communication
// It is tied to an entity can only open connection with entities
type SecureHost interface {
	// Close terminates the `Listen()` function and closes all connections.
	Close() error
	Listen(func(SecureConn)) error
	Open(*ServerIdentity) (SecureConn, error)
	String() string
	WorkingAddress() string
	monitor.CounterIO
}

// SecureConn is the analog of Conn but for secure communication
type SecureConn interface {
	Conn
	ServerIdentity() *ServerIdentity
}

// SecureTCPHost is a TcpHost but with the additional property that it handles
// ServerIdentity.
type SecureTCPHost struct {
	*TCPHost
	// workingAddress is the actual address we're listening. This can
	// be one of the serverIdentity's Addresses or a chosen address if
	// serverIdentity has ":0"-addresses.
	workingAddress string
	// ServerIdentity of this host
	serverIdentity *ServerIdentity
	// Private key tied to this entity
	private abstract.Scalar
	// Lock for accessing this structure
	lockAddress sync.Mutex
	// list of all connections this host has opened
	conns     []*SecureTCPConn
	connMutex sync.Mutex
}

// SecureTCPConn is a secured tcp connection using ServerIdentity as an identity.
type SecureTCPConn struct {
	*TCPConn
	*SecureTCPHost
	serverIdentity *ServerIdentity
}

// Packet is the container for any Msg
type Packet struct {
	// The ServerIdentity of the remote peer we are talking to.
	// Basically, this means that when you open a new connection to someone, and
	// / or listens to incoming connections, the network library will already
	// make some exchange between the two communicants so each knows the
	// ServerIdentity of the others.
	ServerIdentity *ServerIdentity
	// the origin of the message
	From Address
	// What kind of msg do we have
	MsgType PacketTypeID
	// The underlying message
	Msg Body
	// which constructors are used
	Constructors protobuf.Constructors
	// possible error during unmarshalling so that upper layer can know it
	err error
}

// ServerIdentity is used to represent a Conode in the whole internet.
// It's based on a public key, and there can be one or more addresses to contact it.
type ServerIdentity struct {
	// This is the public key of that ServerIdentity
	Public abstract.Point
	// The ServerIdentityID corresponding to that public key
	ID ServerIdentityID
	// A slice of addresses of where that Id might be found
	Address Address
}

// ServerIdentityID uniquely identifies an ServerIdentity struct
type ServerIdentityID uuid.UUID

// Equal returns true if both ServerIdentityID are equal or false otherwise.
func (eid ServerIdentityID) Equal(other ServerIdentityID) bool {
	return uuid.Equal(uuid.UUID(eid), uuid.UUID(other))
}

func (si *ServerIdentity) String() string {
	return si.Address.String()
}

// ServerIdentityType can be used to recognise an ServerIdentity-message
var ServerIdentityType = RegisterPacketType(ServerIdentity{})

// ServerIdentityToml is the struct that can be marshalled into a toml file
type ServerIdentityToml struct {
	Public  string
	Address Address
}

// NewServerIdentity creates a new ServerIdentity based on a public key and with a slice
// of IP-addresses where to find that entity. The Id is based on a
// version5-UUID which can include a URL that is based on it's public key.
func NewServerIdentity(public abstract.Point, address Address) *ServerIdentity {
	url := NamespaceURL + "id/" + public.String()
	return &ServerIdentity{
		Public:  public,
		Address: address,
		ID:      ServerIdentityID(uuid.NewV5(uuid.NamespaceURL, url)),
	}
}

// Equal tests on same public key
func (si *ServerIdentity) Equal(e2 *ServerIdentity) bool {
	return si.Public.Equal(e2.Public)
}

// Toml converts an ServerIdentity to a Toml-structure
func (si *ServerIdentity) Toml(suite abstract.Suite) *ServerIdentityToml {
	var buf bytes.Buffer
	if err := crypto.WritePub64(suite, &buf, si.Public); err != nil {
		log.Error("Error while writing public key:", err)
	}
	return &ServerIdentityToml{
		Address: si.Address,
		Public:  buf.String(),
	}
}

// ServerIdentity converts an ServerIdentityToml structure back to an ServerIdentity
func (si *ServerIdentityToml) ServerIdentity(suite abstract.Suite) *ServerIdentity {
	pub, err := crypto.ReadPub64(suite, strings.NewReader(si.Public))
	if err != nil {
		log.Error("Error while reading public key:", err)
	}
	return &ServerIdentity{
		Public:  pub,
		Address: si.Address,
	}
}

// GlobalBind returns the global-binding address. Given any IP:PORT combination,
// it will return 0.0.0.0:PORT.
func GlobalBind(address string) (string, error) {
	addr := strings.Split(address, ":")
	if len(addr) != 2 {
		return "", errors.New("Not a host:port address")
	}
	return "0.0.0.0:" + addr[1], nil
}

// handleError produces the higher layer error depending on the type
// so user of the package can know what is the cause of the problem
func handleError(err error) error {

	if strings.Contains(err.Error(), "use of closed") || strings.Contains(err.Error(), "broken pipe") {
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

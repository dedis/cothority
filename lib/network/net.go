// This package is a networking library. You have Hosts which can
// issue connections to others hosts, and Conn which are the connections itself.
// Hosts and Conns are interfaces and can be of type Tcp, or Chans, or Udp or
// whatever protocols you think might implement this interface.
// In this library we also provide a way to encode / decode any kind of packet /
// structs. When you want to send a struct to a conn, you first register
// (one-time operation) this packet to the library, and then directly pass the
// struct itself to the conn that will recognize its type. When decoding,
// it will automatically detect the underlying type of struct given, and decode
// it accordingly. You can provide your own decode / encode methods if for
// example, you have a variable length packet structure. For this, just
// implements MarshalBinary or UnmarshalBinary.

package network

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
)

// Network part //

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
	Send(ctx context.Context, obj ProtocolMessage) error
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

// NewTcpHost returns a Fresh TCP Host
// If constructors == nil, it will take an empty one.
func NewTcpHost() *TcpHost {
	return &TcpHost{
		peers:        make(map[string]Conn),
		quit:         make(chan bool),
		constructors: DefaultConstructors(Suite),
	}
}

// Open will create a new connection between this host
// and the remote host named "name". This is a TcpConn.
// If anything went wrong, Conn will be nil.
func (t *TcpHost) Open(name string) (Conn, error) {
	c, err := t.openTcpConn(name)
	if err != nil {
		return nil, err
	}
	t.peers[name] = c
	return c, nil
}

// OpenTcpCOnn is private method that opens a TcpConn to the given name
func (t *TcpHost) openTcpConn(name string) (*TcpConn, error) {
	var err error
	var conn net.Conn
	for i := 0; i < maxRetry; i++ {
		conn, err = net.Dial("tcp", name)
		if err != nil {
			//dbg.Lvl5("(", i, "/", maxRetry, ") Error opening connection to", name)
			time.Sleep(waitRetry)
		} else {
			break
		}
		time.Sleep(waitRetry)
	}
	if conn == nil {
		return nil, fmt.Errorf("Could not connect to %s.", name)
	}
	c := TcpConn{
		Endpoint: name,
		Conn:     conn,
		host:     t,
	}

	return &c, err
}

// Listen for any host trying to contact him.
// Will launch in a goroutine the srv function once a connection is established
func (t *TcpHost) Listen(addr string, fn func(Conn)) error {
	receiver := func(tc *TcpConn) {
		go fn(tc)
	}
	return t.listen(addr, receiver)
}

// listen is the private function that takes a function that takes a TcpConn.
// That way we can control what to do of the TcpConn before returning it to the
// function given by the user. Used by SecureTcpHost
func (t *TcpHost) listen(addr string, fn func(*TcpConn)) error {
	global, _ := cliutils.GlobalBind(addr)
	ln, err := net.Listen("tcp", global)
	if err != nil {
		return fmt.Errorf("Error opening listener on address %s", addr)
	}
	t.listener = ln
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.quit:
				return nil
			default:
			}
			continue
		}
		c := TcpConn{
			Endpoint: conn.RemoteAddr().String(),
			Conn:     conn,
			host:     t,
		}
		t.peers[conn.RemoteAddr().String()] = &c
		fn(&c)
	}
	return nil
}

// Close will close every connection this host has opened
func (t *TcpHost) Close() error {
	if t.closed == true {
		return nil
	}
	t.closed = true
	for _, c := range t.peers {
		if err := c.Close(); err != nil {
			return handleError(err)
		}
	}
	close(t.quit)
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
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

// PeerName returns the name of the peer at the end point of
// the conn
func (c *TcpConn) Remote() string {
	return c.Endpoint
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

// Receive waits for any input on the connection and returns
// the ApplicationMessage **decoded** and an error if something
// wrong occured
func (c *TcpConn) Receive(ctx context.Context) (ApplicationMessage, error) {

	var am ApplicationMessage
	am.Constructors = c.host.constructors
	bufferSize := 4096
	b := make([]byte, bufferSize)
	var buffer bytes.Buffer
	var err error
	//c.Conn.SetReadDeadline(time.Now().Add(timeOut))
	for {
		n, err := c.Conn.Read(b)
		b = b[:n]
		buffer.Write(b)
		if err != nil {
			e := handleError(err)
			return EmptyApplicationMessage, e
		}
		if n < bufferSize {
			// read all data
			break
		}
	}
	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("Error Unmarshalling %s: %dbytes : %v\n", am.MsgType, len(buffer.Bytes()), e)
		}
	}()

	err = am.UnmarshalBinary(buffer.Bytes())
	if err != nil {
		return EmptyApplicationMessage, fmt.Errorf("Error unmarshaling message type %s: %s", am.MsgType.String(), err.Error())
	}
	am.From = c.Remote()
	return am, nil
}

// Send will convert the Protocolmessage into an ApplicationMessage
// Then send the message through the Gob encoder
// Returns an error if anything was wrong
func (c *TcpConn) Send(ctx context.Context, obj ProtocolMessage) error {
	am, err := newApplicationMessage(obj)
	if err != nil {
		return fmt.Errorf("Error converting packet: %v\n", err)
	}
	var b []byte
	b, err = am.MarshalBinary()
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}

	c.Conn.SetWriteDeadline(time.Now().Add(timeOut))
	_, err = c.Conn.Write(b)
	if err != nil {
		return handleError(err)
	}
	return nil
}

// Close ... closes the connection
func (c *TcpConn) Close() error {
	if c.closed == true {
		return nil
	}
	err := c.Conn.Close()
	c.closed = true
	if err != nil {
		return handleError(err)
	}
	return nil
}

// Entity is used to represent a Conode in the whole internet.
// It's kinda an network identity that is based on a public key,
// and there can be one or more addresses to contact it.
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

// First returns the first address available
func (id *Entity) First() string {
	if len(id.Addresses) > 0 {
		return id.Addresses[0]
	}
	return ""
}

// Next returns the next address like an iterator,
// starting at the beginning if nothing worked
func (id *Entity) Next() string {
	addr := id.Addresses[id.iter]
	id.iter = (id.iter + 1) % len(id.Addresses)
	return addr

}

// Equal tests on same public key
func (id *Entity) Equal(e *Entity) bool {
	return id.Public.Equal(e.Public)
}

// NewEntity creates a new Entity based on a public key and with a slice
// of IP-addresses where to find that entity. The Id is based on a
// version5-UUID which can include a URL that is based on it's public key.
func NewEntity(public abstract.Point, addresses ...string) *Entity {
	url := "https://dedis.epfl.ch/id/" + public.String()
	return &Entity{
		Public:    public,
		Addresses: addresses,
		Id:        uuid.NewV5(uuid.NamespaceURL, url),
	}
}

// SecureHost is the analog of Host but with secure communication
// It is tied to an entity can only open connection with identities
type SecureHost interface {
	Close() error
	Listen(func(SecureConn)) error
	Open(id *Entity) (SecureConn, error)
}

// SecureConn is the analog of Conn but for secure comminucation
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

// NewSecureTcpHost returns a Secure Tcp Host
func NewSecureTcpHost(private abstract.Secret, id *Entity) *SecureTcpHost {
	return &SecureTcpHost{
		private:        private,
		entity:         id,
		EntityToAddr:   make(map[uuid.UUID]string),
		TcpHost:        NewTcpHost(),
		workingAddress: id.First(),
	}
}

// Listen will try each addresses it the host Entity.
// Returns an error if it can listen on any address
func (st *SecureTcpHost) Listen(fn func(SecureConn)) error {
	receiver := func(c *TcpConn) {
		dbg.Lvl3("Listening on", c.Remote())
		stc := &SecureTcpConn{
			TcpConn:       c,
			SecureTcpHost: st,
		}
		// if negociation fails we drop the connection
		if err := stc.negotiateListen(); err != nil {
			dbg.Error("Negociation failed:", err)
			stc.Close()
			return
		}
		go fn(stc)
	}
	var addr string
	dbg.Lvl3("Addresses are", st.entity.Addresses)
	for _, addr = range st.entity.Addresses {
		dbg.Lvl3("Trying to listen on", addr)
		st.workingAddress = addr
		if err := st.TcpHost.listen(addr, receiver); err != nil {
			// The listening is over
			if err == ErrClosed || err == ErrEOF {
				return nil
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("No address worked for listening on this host")
}

// Open will try any address that is in the Entity and connect to the first
// one that works. Then it exchanges the Entity to verify.
func (st *SecureTcpHost) Open(id *Entity) (SecureConn, error) {
	var secure SecureTcpConn
	var success bool
	// try all names
	for _, addr := range id.Addresses {
		// try to connect with this name
		c, err := st.TcpHost.openTcpConn(addr)
		if err != nil {
			continue
		}
		// create the secure connection
		secure = SecureTcpConn{
			TcpConn:       c,
			SecureTcpHost: st,
			entity:        id,
		}
		success = true
		break
	}
	if !success {
		return nil, fmt.Errorf("Could not connect to any address tied to this Entity")
	}
	// Exchange and verify Identities
	return &secure, secure.negotiateOpen(id)
}

type SecureTcpConn struct {
	*TcpConn
	*SecureTcpHost
	entity *Entity
}

// negotitateListen is made to exchange the Entity between the two parties.
// when a connection request is made during listening
func (sc *SecureTcpConn) negotiateListen() error {
	// Send our Entity to the remote endpoint
	if err := sc.TcpConn.Send(context.TODO(), sc.SecureTcpHost.entity); err != nil {
		return fmt.Errorf("Error while sending indentity during negotiation:%s", err)
	}
	// Receive the other Entity
	nm, err := sc.TcpConn.Receive(context.TODO())
	if err != nil {
		return fmt.Errorf("Error while receiving Entity during negotiation %s", err)
	}
	// Check if it is correct
	if nm.MsgType != EntityType {
		return fmt.Errorf("Received wrong type during negotiation %s", nm.MsgType.String())
	}

	// Set this ID for this connection
	id := nm.Msg.(Entity)
	sc.entity = &id
	return nil
}

// negotiateOpen is called when Open a connection is called. Plus
// negotiateListen it also verifiy the Entity.
func (sc *SecureTcpConn) negotiateOpen(id *Entity) error {
	if err := sc.negotiateListen(); err != nil {
		return err
	}

	// verify the Entity if its the same we are supposed to connect
	if sc.Entity().Id != id.Id {
		return fmt.Errorf("Entity received during negotiation is wrong. WARNING")
	}

	return nil
}

// Receive is analog to Conn.Receive but also set the right Entity in the
// message
func (sc *SecureTcpConn) Receive(ctx context.Context) (ApplicationMessage, error) {
	nm, err := sc.TcpConn.Receive(ctx)
	nm.Entity = sc.entity
	return nm, err
}

func (sc *SecureTcpConn) Entity() *Entity {
	return sc.entity
}

func init() {
	RegisterProtocolType(EntityType, Entity{})
}

// EntityToml is the struct that can be marshalled into a toml file
type EntityToml struct {
	Public    string
	Addresses []string
}

func (id *Entity) Toml(suite abstract.Suite) *EntityToml {
	var buf bytes.Buffer
	cliutils.WritePub64(suite, &buf, id.Public)
	return &EntityToml{
		Addresses: id.Addresses,
		Public:    buf.String(),
	}
}
func (id *EntityToml) Entity(suite abstract.Suite) *Entity {
	pub, _ := cliutils.ReadPub64(suite, strings.NewReader(id.Public))
	return &Entity{
		Public:    pub,
		Addresses: id.Addresses,
	}
}

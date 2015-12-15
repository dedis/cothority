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
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"time"

	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/protobuf"
)

/// Encoding part ///

type Type uint8

var TypeRegistry = make(map[Type]reflect.Type)
var InvTypeRegistry = make(map[reflect.Type]Type)

var globalOrder = binary.LittleEndian

// RegisterProtocolType register a custom "struct" / "packet" and get
// the allocated Type
// Pass simply an your non-initialized struct
func RegisterProtocolType(msgType Type, msg ProtocolMessage) error {
	if _, typeRegistered := TypeRegistry[msgType]; typeRegistered {
		return errors.New("Type was already registered")
	}
	t := reflect.TypeOf(msg)
	TypeRegistry[msgType] = t
	InvTypeRegistry[t] = msgType

	return nil
}

// String returns the underlying type in human format
func (t Type) String() string {
	ty, ok := TypeRegistry[t]
	if !ok {
		return "unknown"
	}
	return ty.Name()
}

// ProtocolMessage is a type for any message that the user wants to send
type ProtocolMessage interface{}

// ApplicationMessage is the container for any ProtocolMessage
type ApplicationMessage struct {
	// From field can be set by the receivinf connection itself, no need to
	// acutally transmit the value
	From string
	// What kind of msg do we have
	MsgType Type
	// The underlying message
	Msg ProtocolMessage

	constructors protobuf.Constructors
}

// MarshalBinary the application message => to bytes
// Implements BinaryMarshaler interface so it will be used when sending with protobuf
func (am *ApplicationMessage) MarshalBinary() ([]byte, error) {
	b := new(bytes.Buffer)
	if err := binary.Write(b, globalOrder, am.MsgType); err != nil {
		return nil, err
	}
	var buf []byte
	var err error
	if buf, err = protobuf.Encode(&am.Msg); err != nil {
		return nil, err
	}
	_, err = b.Write(buf)
	return b.Bytes(), err
}

// UnmarshalBinary will decode the incoming bytes
// It checks if the underlying packet is self-decodable
// by using its UnmarshalBinary interface
// otherwise, use abstract.Encoding (suite) to decode
func (am *ApplicationMessage) UnmarshalBinary(buf []byte) error {
	dbg.Print("UnmarshalBinary called")
	b := bytes.NewBuffer(buf)
	if err := binary.Read(b, globalOrder, &am.MsgType); err != nil {
		return err
	}

	if typ, ok := TypeRegistry[am.MsgType]; !ok {
		return fmt.Errorf("Type %s not registered.", am.MsgType.String())
	} else {
		ptr := reflect.New(typ)
		var err error
		if err = protobuf.DecodeWithConstructors(b.Bytes(), ptr, am.constructors); err != nil {
			return err
		}
		am.Msg = ptr.Elem().Interface()
	}
	return nil
}

// ConstructFrom takes a ProtocolMessage and then construct a
// ApplicationMessage from it. Error if the type is unknown
func (am *ApplicationMessage) ConstructFrom(obj ProtocolMessage) error {
	t := reflect.TypeOf(obj)
	ty, ok := InvTypeRegistry[t]
	if !ok {
		return errors.New(fmt.Sprintf("Packet to send is not known. Please register packet: %s\n", t.String()))
	}
	am.MsgType = ty
	am.Msg = obj
	return nil
}

// Network part //

// How many times should we try to connect
const maxRetry = 10
const waitRetry = 1 * time.Second

// Host is the basic interface to represent a Host of any kind
// Host can open new Conn(ections) and Listen for any incoming Conn(...)
type Host interface {
	Name() string
	Open(name string) Conn
	Listen(addr string, fn func(Conn)) // the srv processing function
}

// Conn is the basic interface to represent any communication mean
// between two host. It is closely related to the underlying type of Host
// since a TcpHost will generate only TcpConn
type Conn interface {
	PeerName() string
	Send(ctx context.Context, obj ProtocolMessage) error
	Receive(ctx context.Context) (ApplicationMessage, error)
	Close()
}

// TcpHost is the underlying implementation of
// Host using Tcp as a communication channel
type TcpHost struct {
	// its name (usually its IP address)
	name string
	// A list of connection maintained by this host
	peers map[string]Conn
	// a list of constructors for en/decoding
	constructors protobuf.Constructors
}

// TcpConn is the underlying implementation of
// Conn using Tcp
type TcpConn struct {
	// Peer is the name of the endpoint
	Peer string

	// The connection used
	Conn net.Conn

	// A pointer to the associated host (just-in-case)
	host *TcpHost
}

// PeerName returns the name of the peer at the end point of
// the conn
func (c *TcpConn) PeerName() string {
	return c.Peer
}

// Receive waits for any input on the connection and returns
// the ApplicationMessage **decoded** and an error if something
// wrong occured
func (c *TcpConn) Receive(ctx context.Context) (ApplicationMessage, error) {
	dbg.Print("func (c *TcpConn) Receive() called")
	var am ApplicationMessage
	am.constructors = c.host.constructors
	//b:= make(bytes[])
	var b []byte
	var err error
	dbg.Print("Before readall")
	b, err = ioutil.ReadAll(c.Conn)
	//c.Conn.SetReadDeadline(time.Now().Add(time.Second))
	if err != nil {
		dbg.Fatal("Could not read/decode from connection", err)
		return am, err
	}
	err = am.UnmarshalBinary(b)
	if err != nil {
		dbg.Fatal("Could not read/decode from connection", err)
		return am, err
	}

	return am, nil
}

// Send will convert the Protocolmessage into an ApplicationMessage
// Then send the message through the Gob encoder
// Returns an error if anything was wrong
func (c *TcpConn) Send(ctx context.Context, obj ProtocolMessage) error {
	am := ApplicationMessage{}
	err := am.ConstructFrom(obj)
	if err != nil {
		return fmt.Errorf("Error converting packet: %v\n", err)
	}
	var b []byte
	dbg.Print("Before: MarshalBinary()")
	b, err = am.MarshalBinary()
	if err != nil {
		return fmt.Errorf("Error sending message: %v", err)
	}
	dbg.Print("After: MarshalBinary()", len(b), "bytes")

	var num int
	//c.Conn.SetWriteDeadline(time.Now().Add(time.Second))
	num, err = c.Conn.Write(b)
	dbg.Print("Wrote number of bytes:", num)
	return err
}

// Close ... closes the connection
func (c *TcpConn) Close() {
	err := c.Conn.Close()
	if err != nil {
		dbg.Fatal("Error while closing tcp conn:", err)
	}
}

// NewTcpHost returns a Fresh TCP Host
func NewTcpHost(name string, constructors protobuf.Constructors) *TcpHost {
	return &TcpHost{
		name:         name,
		peers:        make(map[string]Conn),
		constructors: constructors,
	}
}

// Name is the name ofthis host
func (t *TcpHost) Name() string {
	return t.name
}

// Open will create a new connection between this host
// and the remote host named "name". This is a TcpConn.
// If anything went wrong, Conn will be nil.
func (t *TcpHost) Open(name string) Conn {
	var conn net.Conn
	var err error
	for i := 0; i < maxRetry; i++ {

		conn, err = net.Dial("tcp", name)
		if err != nil {
			dbg.Lvl3(t.Name(), "(", i, "/", maxRetry, ") Error opening connection to", name)
			time.Sleep(waitRetry)
		} else {
			break
		}
		time.Sleep(waitRetry)
	}
	if conn == nil {
		dbg.Error(t.Name(), "could not connect to", name, ": ABORT")
		return nil
	}
	c := TcpConn{
		Peer: name,
		Conn: conn,
		host: t,
	}
	t.peers[name] = &c
	return &c
}

// Listen for any host trying to contact him.
// Will launch in a goroutine the srv function once a connection is established
func (t *TcpHost) Listen(addr string, fn func(Conn)) {
	global, _ := cliutils.GlobalBind(addr)
	ln, err := net.Listen("tcp", global)
	if err != nil {
		dbg.Fatal("error listening (host", t.Name(), ")")
	}
	dbg.Lvl3(t.Name(), "Waiting for connections on addr", addr, "..\n")
	for {
		conn, err := ln.Accept()
		if err != nil {
			dbg.Lvl2(t.Name(), "error accepting connection:", err)
			continue
		}
		c := TcpConn{
			Peer: conn.RemoteAddr().String(),
			Conn: conn,
			host: t,
		}
		t.peers[conn.RemoteAddr().String()] = &c
		go fn(&c)
	}
}

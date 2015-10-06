package network

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"net"
	"os"
	"reflect"
)

/// Encoding part ///

type Type uint8

var currType Type
var Suite abstract.Suite
var TypeRegistry = make(map[Type]reflect.Type)
var InvTypeRegistry = make(map[reflect.Type]Type)

// Register a custom "struct" / "packet" and get
// the allocated Type
// Pass simply an your non-initialized struct
func RegisterProtocolType(msg ProtocolMessage) Type {
	currType += 1
	t := reflect.TypeOf(msg)
	TypeRegistry[currType] = t
	InvTypeRegistry[t] = currType
	return currType
}

func init() {
}

func (t Type) String() string {
	ty, ok := TypeRegistry[t]
	if !ok {
		return "unknown"
	}
	return ty.Name()
}

type ProtocolMessage interface{}

type ApplicationMessage struct {
	MsgType Type
	Msg     ProtocolMessage
}

// Implements BinaryMarshaler interface so it will be used when sending with gob
func (am *ApplicationMessage) MarshalBinary() ([]byte, error) {
	var buf = new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, am.MsgType)
	if err != nil {
		return nil, err
	}
	err = Suite.Write(buf, am.Msg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (am *ApplicationMessage) UnmarshalBinary(buf []byte) error {
	b := bytes.NewBuffer(buf)
	var t Type
	err := binary.Read(b, binary.BigEndian, &t)
	if err != nil {
		fmt.Printf("Error reading Type : %v\n", err)
		os.Exit(1)
	}

	ty, ok := TypeRegistry[t]
	if !ok {
		fmt.Printf("Type %d is not registered so we can not allocate this type %s\n", t, t.String())
		os.Exit(1)
	}
	v := reflect.New(ty).Elem()
	err = Suite.Read(b, v.Addr().Interface())
	if err != nil {
		fmt.Printf("Error decoding ProtocolMessage : %v\n", err)
		os.Exit(1)
	}
	am.MsgType = t
	am.Msg = v.Interface()
	//fmt.Printf("UnmarshalBinary() : Decoded type %s => %v\n", t.String(), ty)
	return nil
}

func (am *ApplicationMessage) ConstructFrom(obj ProtocolMessage) error {
	t := reflect.TypeOf(obj)
	ty, ok := InvTypeRegistry[t]
	if !ok {
		return errors.New(fmt.Sprintf("Packet to send is not known. Please register packet : %s\n", t.String()))
	}
	am.MsgType = ty
	am.Msg = obj
	return nil
}

// Network part //

const ListenPort = "5000"

// How many times should we try to connect
const maxRetry = 5

type Host interface {
	Name() string
	Open(name string) Conn
	Listen(addr string, fn func(Conn)) // the srv processing function
}

type Conn interface {
	PeerName() string
	Send(obj ProtocolMessage) error
	Receive() (ApplicationMessage, error)
	Close()
}

type TcpHost struct {
	name string

	peers map[string]Conn
}

type TcpConn struct {
	Peer string

	Conn net.Conn
	enc  *gob.Encoder
	dec  *gob.Decoder
	host *TcpHost
}

func (c *TcpConn) PeerName() string {
	return c.Peer
}
func (c *TcpConn) Receive() (ApplicationMessage, error) {
	var am ApplicationMessage
	err := c.dec.Decode(&am)
	if err != nil {
		fmt.Printf("Error decoding ApplicationMessage : %v\n", err)
		os.Exit(1)
	}
	return am, nil
}

func (c *TcpConn) Send(obj ProtocolMessage) error {
	am := ApplicationMessage{}
	err := am.ConstructFrom(obj)
	if err != nil {
		fmt.Printf("Error converting packet : %v\n", err)
		os.Exit(1) // should not happen . I know.
	}
	err = c.enc.Encode(&am)
	if err != nil {
		fmt.Printf("Error sending application message ..")
		os.Exit(1)
	}
	return err
}

func (c *TcpConn) Close() {
	err := c.Conn.Close()
	if err != nil {
		fmt.Printf("Error while closing tcp conn : %v\n", err)
		os.Exit(1)
	}
}

func NewTcpHost(name string) *TcpHost {
	return &TcpHost{
		name:  name,
		peers: make(map[string]Conn),
	}
}

func (t *TcpHost) Name() string {
	return t.name
}
func (t *TcpHost) Open(name string) Conn {
	var conn net.Conn
	var err error
	for i := 0; i < maxRetry; i++ {

		conn, err = net.Dial("tcp", name)
		if err != nil {
			fmt.Printf("Error opening connection to %s\n", name)
		} else {
			break
		}
	}
	if conn == nil {
		os.Exit(1)
	}
	c := TcpConn{
		Peer: name,
		Conn: conn,
		enc:  gob.NewEncoder(conn),
		dec:  gob.NewDecoder(conn),
		host: t,
	}
	t.peers[name] = &c
	return &c
}

// Listen for any host trying to contact him.
// Will launch in a goroutine the srv function once a connection is established
func (t *TcpHost) Listen(addr string, fn func(Conn)) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("error listening (host %s)\n", t.name)
	}
	dbg.Lvl3(t.Name(), "Waiting for connections on addr ", addr, "..\n")
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection : %v\n", err)
			os.Exit(1)
		}
		c := TcpConn{
			Peer: conn.RemoteAddr().String(),
			Conn: conn,
			enc:  gob.NewEncoder(conn),
			dec:  gob.NewDecoder(conn),
			host: t,
		}
		t.peers[conn.RemoteAddr().String()] = &c
		go fn(&c)
	}
}

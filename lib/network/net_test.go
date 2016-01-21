package network

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/satori/go.uuid"
)

// Some packet and their respective network type
type TestMessage struct {
	Point  abstract.Point
	Secret abstract.Secret
}

type PublicPacket struct {
	Point abstract.Point
}

// The tSuite we use
var tSuite = Suite

// Here we registers the packets, so that the decoder can instantiate
// to the right type and then we can do event-driven stuff such as receiving
// new messages without knowing the type and then check on the MsgType field
// to cast to the right packet type (See below)
var PublicType = RegisterMessageType(PublicPacket{})
var TestMessageType = RegisterMessageType(TestMessage{})

type TestRegisterS struct {
	I int
}

func TestRegister(t *testing.T) {
	if TypeFromData(&TestRegisterS{}) != ErrorType {
		t.Fatal("TestRegister should not yet be there")
	}

	trType := RegisterMessageType(&TestRegisterS{})
	if uuid.Equal(trType, uuid.Nil) {
		t.Fatal("Couldn't register TestRegister-struct")
	}

	if TypeFromData(&TestRegisterS{}) != trType {
		t.Fatal("TestRegister is different now")
	}
	if TypeFromData(TestRegisterS{}) != trType {
		t.Fatal("TestRegister is different now")
	}
}

// Test closing and opening of Host on same address
func TestMultiClose(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	fn := func(s Conn) {
		dbg.Lvl3("Getting connection from", s)
	}
	h1 := NewTcpHost()
	h2 := NewTcpHost()
	go h1.Listen("localhost:2000", fn)
	h2.Open("localhost:2000")
	err := h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	dbg.Lvl3("Finished first connection, starting 2nd")
	h1 = NewTcpHost()
	go func() {
		err := h1.Listen("localhost:2000", fn)
		if err != nil {
			t.Fatal("Couldn't re-open listener")
		}
	}()
	time.Sleep(time.Millisecond * 100)
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close h1:", err)
	}
}

// Test closing and opening of SecureHost on same address
func TestSecureMultiClose(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	fn := func(s SecureConn) {
		dbg.Lvl3("Getting connection from", s)
	}

	priv1, pub1 := config.NewKeyPair(Suite)
	entity1 := NewEntity(pub1, "localhost:2000")
	priv2, pub2 := config.NewKeyPair(Suite)
	entity2 := NewEntity(pub2, "localhost:2001")

	h1 := NewSecureTcpHost(priv1, entity1)
	h2 := NewSecureTcpHost(priv2, entity2)
	go func() {
		err := h1.Listen(fn)
		if err != nil {
			t.Fatal("Listening failed for h1:", err)
		}
	}()
	h2.Open(entity1)
	err := h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	dbg.Lvl3("Finished first connection, starting 2nd")
	h1 = NewSecureTcpHost(priv1, entity1)
	go func() {
		err = h1.Listen(fn)
		if err != nil {
			t.Fatal("Couldn't re-open listener")
		}
	}()
	time.Sleep(time.Millisecond * 100)
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close h1:", err)
	}
}

// Testing exchange of entity
func TestSecureTcp(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	fn := func(s SecureConn) {
		dbg.Lvl3("Getting connection from", s)
	}

	priv1, pub1 := config.NewKeyPair(Suite)
	entity1 := NewEntity(pub1, "localhost:2000")
	priv2, pub2 := config.NewKeyPair(Suite)
	entity2 := NewEntity(pub2, "localhost:2001")

	host1 := NewSecureTcpHost(priv1, entity1)
	host2 := NewSecureTcpHost(priv2, entity2)

	go host1.Listen(fn)
	conn, err := host2.Open(entity1)
	if err != nil {
		t.Fatal("Couldn't connect to host1:", err)
	}
	if !conn.Entity().Public.Equal(pub1) {
		t.Fatal("Connection-id is not from host1")
	}
	host1.Close()
	host2.Close()
}

// Testing a full-blown server/client
func TestTcpNetwork(t *testing.T) {
	// Create one client + one server
	clientHost := NewTcpHost()
	serverHost := NewTcpHost()
	// Give them keys
	clientPub := tSuite.Point().Base()
	serverPub := tSuite.Point().Add(tSuite.Point().Base(), tSuite.Point().Base())
	wg := sync.WaitGroup{}
	client := NewSimpleClient(clientHost, clientPub, &wg)
	server := NewSimpleServer(serverHost, serverPub, t, &wg)
	// Make the server listens
	go server.Listen("127.0.0.1:5000", server.ExchangeWithClient)
	time.Sleep(1 * time.Second)
	// Make the client engage with the server
	client.ExchangeWithServer("127.0.0.1:5000", t)
	wg.Wait()
	clientHost.Close()
	serverHost.Close()
}

type SimpleClient struct {
	Host
	Pub   abstract.Point
	Peers []abstract.Point
	wg    *sync.WaitGroup
}

// The server
type SimpleServer struct {
	Host
	Pub abstract.Point
	t   *testing.T
	wg  *sync.WaitGroup
}

// Create a new simple server
func NewSimpleServer(host Host, pub abstract.Point, t *testing.T, wg *sync.WaitGroup) *SimpleServer {
	s := &SimpleServer{}
	s.Host = host
	s.Pub = pub
	s.t = t
	s.wg = wg
	return s
}

// Createa a new simple client
func NewSimpleClient(host Host, pub abstract.Point, wg *sync.WaitGroup) *SimpleClient {
	return &SimpleClient{
		Host:  host,
		Pub:   pub,
		Peers: make([]abstract.Point, 0),
		wg:    wg,
	}
}

// overridding Name host
func (s *SimpleClient) Name() string {
	return "Client "
}

// Simplest protocol : exchange keys with the server
func (s *SimpleClient) ExchangeWithServer(name string, t *testing.T) {
	s.wg.Add(1)
	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	defer func() {
		//dbg.Print("ExchangeWithServer canceld/timed out")
		//cancel()
	}()

	// open a connection to the peer
	c, err := s.Open(name)
	if err != nil {
		t.Fatal("client connection is nil ><")
	}
	// create pack
	p := PublicPacket{
		Point: s.Pub,
	}
	// Send it
	err = c.Send(ctx, &p)
	if err != nil {
		t.Fatal("error sending from client:", err)
	}
	// Receive the response
	am, err := c.Receive(ctx)
	if err != nil {
		fmt.Printf("error receiving ..")
	}

	// Cast to the right type
	if am.MsgType != PublicType {
		t.Fatal("Received a non-wanted packet.\n")
	}

	c.Close()
	s.wg.Done()
}

func (s *SimpleServer) Name() string {
	return "Server "
}

func (s *SimpleServer) ProxySend(c Conn, msg ProtocolMessage) {
	ctx := context.TODO()
	if err := c.Send(ctx, msg); err != nil {
		s.t.Fatal(err)
	}
}

// this is the callback when a new connection is don
func (s *SimpleServer) ExchangeWithClient(c Conn) {
	s.wg.Add(1)
	p := PublicPacket{
		Point: s.Pub,
	}

	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	defer func() {
		//dbg.Print("Canceling because of timeout")
		//cancel()
	}()

	s.ProxySend(c, &p)
	am, err := c.Receive(ctx)
	if err != nil {
		s.t.Error("Server errored when receiving packet ...\n")
	}
	if am.MsgType != PublicType {
		s.t.Error("Server received a non-wanted packet\n")
	}
	p = (am.Msg).(PublicPacket)
	comp := tSuite.Point().Base()
	if !p.Point.Equal(comp) {
		s.t.Error("point not equally reconstructed")
	}
	c.Close()
	s.wg.Done()
}

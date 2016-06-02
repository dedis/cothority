package network

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"os"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/satori/go.uuid"
)

type PublicPacket struct {
	Point abstract.Point
}

// Here we registers the packets, so that the decoder can instantiate
// to the right type and then we can do event-driven stuff such as receiving
// new messages without knowing the type and then check on the MsgType field
// to cast to the right packet type (See below)
var PublicType = RegisterMessageType(PublicPacket{})

type TestRegisterS struct {
	I int
}

func TestMain(m *testing.M) {
	code := m.Run()
	dbg.AfterTest(nil)
	os.Exit(code)
}

func TestRegister(t *testing.T) {
	defer dbg.AfterTest(t)
	if TypeFromData(&TestRegisterS{}) != ErrorType {
		t.Fatal("TestRegister should not yet be there")
	}

	trType := RegisterMessageType(&TestRegisterS{})
	if uuid.Equal(uuid.UUID(trType), uuid.Nil) {
		t.Fatal("Couldn't register TestRegister-struct")
	}

	if TypeFromData(&TestRegisterS{}) != trType {
		t.Fatal("TestRegister is different now")
	}
	if TypeFromData(TestRegisterS{}) != trType {
		t.Fatal("TestRegister is different now")
	}
}

func TestRegisterReflect(t *testing.T) {
	defer dbg.AfterTest(t)

	typ := RegisterMessageType(TestRegisterS{})
	typReflect := RTypeToMessageTypeID(reflect.TypeOf(TestRegisterS{}))
	if typ != typReflect {
		t.Fatal("Register does not work")
	}
}

// Test closing and opening of Host on same address
func TestMultiClose(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 4)
	gotConnect := make(chan bool)
	fn := func(s Conn) {
		dbg.Lvl3("Getting connection from", s)
		gotConnect <- true
	}
	h1 := NewTCPHost()
	h2 := NewTCPHost()
	done := make(chan bool)
	go func() {
		err := h1.Listen("localhost:2000", fn)
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		done <- true
	}()
	time.Sleep(time.Second)
	dbg.Lvl3("Open connection to h2")
	_, err := h2.Open("localhost:2000")
	if err != nil {
		t.Fatal(h2, "couldn't Open() connection to", h1, err)
	}
	// wait for the listener, then close h1 & h2:
	<-gotConnect
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't Close():", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't Close()", err)
	}
	<-done

	h3 := NewTCPHost()
	go func() {
		err := h3.Listen("localhost:2000", fn)
		if err != nil {
			t.Fatal("Couldn't re-open listener:", err)
		}
		done <- true
	}()
	_, err = h2.Open("localhost:2000")
	if err != nil {
		t.Fatal(h2, "couldn't Open() connection to", h3, err)
	}
	// wait for the listener and close h3 & h2
	<-gotConnect
	err = h3.Close()
	if err != nil {
		t.Fatal("Couldn't close h3:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close h2:", err)
	}
	<-done
}

// Test closing and opening of SecureHost on same address
func TestSecureMultiClose(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 4)
	receiverStarted := make(chan bool)
	fn := func(s SecureConn) {
		dbg.Lvl3("Getting connection from", s.Entity().First())
		close(receiverStarted)
	}

	kp1 := config.NewKeyPair(Suite)
	entity1 := NewEntity(kp1.Public, "localhost:2000")
	//entity3 := NewEntity(kp1.Public, "localhost:2000")
	kp2 := config.NewKeyPair(Suite)
	entity2 := NewEntity(kp2.Public, "localhost:2001")

	h1 := NewSecureTCPHost(kp1.Secret, entity1)
	h2 := NewSecureTCPHost(kp2.Secret, entity2)
	done := make(chan bool)
	go func() {
		err := h1.Listen(fn)
		if err != nil {
			t.Fatal("Listening failed for h1:", err)
		}
		done <- true
	}()

	_, err := h2.Open(entity1)
	if err != nil {
		t.Fatal("Couldn't open h2:", err)
	}
	<-receiverStarted
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	<-done

	dbg.Lvl3("Finished first connection, starting 2nd")
	receiverStarted2 := make(chan bool)
	fn2 := func(s SecureConn) {
		dbg.Lvl3("Getting connection from", s.Entity().First())
		receiverStarted2 <- true
	}
	done2 := make(chan bool)
	go func() {
		err := h1.Listen(fn2)
		if err != nil {
			t.Fatal("Couldn't re-open listener:", err)
		}
		done2 <- true
	}()
	_, err = h2.Open(h1.entity)
	if err != nil {
		t.Fatal(err)
	}

	<-receiverStarted2
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close h1:", err)
	}

	<-done2
}

func TestTcpCounterIO(t *testing.T) {
	defer dbg.AfterTest(t)

	RegisterMessageType(&TestRegisterS{})
	dbg.TestOutput(testing.Verbose(), 4)
	receiverStarted := make(chan bool)
	fn := func(s Conn) {
		err := s.Send(context.TODO(), &TestRegisterS{10})
		if err != nil {
			t.Fatal("Error while sending message:", err)
		}
		close(receiverStarted)
	}

	h1 := NewTCPHost()
	h2 := NewTCPHost()
	done := make(chan bool)
	go func() {
		err := h1.Listen("localhost:3000", fn)
		if err != nil {
			t.Fatal("Listening failed for h1:", err)
		}
		done <- true
	}()

	c2, err := h2.Open("localhost:3000")
	if err != nil {
		t.Fatal("Couldn't open h2:", err)
	}
	<-receiverStarted
	c2.Receive(context.TODO())
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	<-done
	// verify the amount of bytes read / written
	if h1.Tx() == 0 || h1.Tx() != h2.Rx() || h2.Rx() == 0 || h2.Rx() != c2.Rx() {
		t.Fatal("stg is wrong with CounterIO implementation of TcpConn / TcpHost")
	}

}

// Testing exchange of entity
func TestSecureTcp(t *testing.T) {
	defer dbg.AfterTest(t)

	dbg.TestOutput(testing.Verbose(), 4)
	opened := make(chan bool)
	fn := func(s SecureConn) {
		dbg.Lvl3("Getting connection from", s)
		opened <- true
	}

	kp1 := config.NewKeyPair(Suite)
	entity1 := NewEntity(kp1.Public, "localhost:2000")
	kp2 := config.NewKeyPair(Suite)
	entity2 := NewEntity(kp2.Public, "localhost:2001")

	host1 := NewSecureTCPHost(kp1.Secret, entity1)
	host2 := NewSecureTCPHost(kp1.Secret, entity2)

	done := make(chan bool)
	go func() {
		err := host1.Listen(fn)
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		done <- true
	}()
	conn, err := host2.Open(entity1)
	if err != nil {
		t.Fatal("Couldn't connect to host1:", err)
	}
	if !conn.Entity().Public.Equal(kp1.Public) {
		t.Fatal("Connection-id is not from host1")
	}
	if !<-opened {
		t.Fatal("Lazy programmers - no select")
	}
	dbg.Lvl3("Closing connections")
	if err := host1.Close(); err != nil {
		t.Fatal("Couldn't close host", host1)
	}
	if err := host2.Close(); err != nil {
		t.Fatal("Couldn't close host", host2)
	}
	<-done
}

// Testing a full-blown server/client
func TestTcpNetwork(t *testing.T) {
	defer dbg.AfterTest(t)

	// Create one client + one server
	clientHost := NewTCPHost()
	serverHost := NewTCPHost()
	// Give them keys
	clientPub := Suite.Point().Base()
	serverPub := Suite.Point().Add(Suite.Point().Base(), Suite.Point().Base())
	wg := sync.WaitGroup{}
	client := NewSimpleClient(clientHost, clientPub, &wg)
	server := NewSimpleServer(serverHost, serverPub, t, &wg)
	// Make the server listen
	done := make(chan bool)
	go func() {
		err := server.Listen("127.0.0.1:5000", server.ExchangeWithClient)
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		done <- true
	}()
	// Make the client engage with the server
	client.ExchangeWithServer("127.0.0.1:5000", t)
	wg.Wait()
	if err := clientHost.Close(); err != nil {
		t.Fatal("could not close client", err)
	}
	if err := serverHost.Close(); err != nil {
		t.Fatal("could not close server", err)
	}
	<-done
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
	err = c.Close()
	if err != nil {
		t.Fatal("error closing connection", err)
	}

	err = c.Close()
	if err != nil && err != ErrClosed {
		t.Fatal("Couldn't close:", err)
	}
	if err == ErrClosed {
		dbg.Error("Called Close() on alredy closed connetion.")
	}
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
	s.ProxySend(c, &p)
	am, err := c.Receive(ctx)
	if err != nil {
		s.t.Error("Server errored when receiving packet ...\n")
	}
	if am.MsgType != PublicType {
		s.t.Error("Server received a non-wanted packet\n")
	}
	p = (am.Msg).(PublicPacket)
	comp := Suite.Point().Base()
	if !p.Point.Equal(comp) {
		s.t.Error("point not equally reconstructed")
	}
	err = c.Close()
	if err != nil {
		s.t.Fatal("error closing connection", err)
	}

	s.wg.Done()
}

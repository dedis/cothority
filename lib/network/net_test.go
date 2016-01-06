package network

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
)

// Some packet and their respective network type
type TestMessage struct {
	Point  abstract.Point
	Secret abstract.Secret
}

var TestMessageType Type = 4

type PublicPacket struct {
	Point abstract.Point
}

var PublicType Type = 5

// The suite we use
var suite = edwards.NewAES128SHA256Ed25519(false)

func init() {
	// Here we registers the packets themself so the decoder can instantiate
	// to the right type and then we can do event-driven stuff such as receiving
	// new messages without knowing the type and then check on the MsgType field
	// to cast to the right packet type (See below)
	RegisterProtocolType(PublicType, PublicPacket{})
	RegisterProtocolType(TestMessageType, TestMessage{})
}

// The test function
func TestTcpNetwork(t *testing.T) {
	// Create one client + one server
	clientHost := NewTcpHost(DefaultConstructors(suite))
	serverHost := NewTcpHost(DefaultConstructors(suite))
	// Give them keys
	clientPub := suite.Point().Base()
	serverPub := suite.Point().Add(suite.Point().Base(), suite.Point().Base())
	wg := sync.WaitGroup{}
	client := NewSimpleClient(clientHost, clientPub, &wg)
	server := NewSimpleServer(serverHost, serverPub, t, &wg)
	// Make the server listens
	go server.Listen("127.0.0.1:5000", server.ExchangeWithClient)
	time.Sleep(1 * time.Second)
	// Make the client engage with the server
	client.ExchangeWithServer("127.0.0.1:5000", t)
	wg.Wait()
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
	comp := suite.Point().Base()
	if !p.Point.Equal(comp) {
		s.t.Error("point not equally reconstructed")
	}
	c.Close()
	s.wg.Done()
}

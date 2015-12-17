package network

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/protobuf"
)

type TestMessage struct {
	Point  abstract.Point
	Secret abstract.Secret
}

var TestMessageType Type = 4

type PublicPacket struct {
	Point abstract.Point
}

var PublicType Type = 5

var constructors protobuf.Constructors

var suite = edwards.NewAES128SHA256Ed25519(false)

func init() {
	// Here we are using the protobuf.Constrcutors mechanisms. So when we
	// encounters a non basic type, we first check if we have a constructor for
	// it, if yes, calls it to get a initialized object, otherwise just call
	// reflect.New(type). This enfors the use of only ONE suite for example
	// for all the connections related to one host. This behavior can maybe
	// change in the future with the use of the Context thing currently
	// implementing in crypto branch cipher
	var suite = edwards.NewAES128SHA256Ed25519(false)
	constructors = make(protobuf.Constructors)
	var point abstract.Point
	var secret abstract.Secret
	constructors[reflect.TypeOf(&point).Elem()] = func() interface{} { return suite.Point() }
	constructors[reflect.TypeOf(&secret).Elem()] = func() interface{} { return suite.Secret() }
	dbg.Print("Point/Secret constructors added to global var")

	// Here we registers the packets themself so the decoder can instantiate
	// to the right type and then we can do event-driven stuff such as receiving
	// new messages without knowing the type and then check on the MsgType field
	// to cast to the right packet type (See below)
	RegisterProtocolType(PublicType, PublicPacket{})
	RegisterProtocolType(TestMessageType, TestMessage{})
}

type SimpleClient struct {
	Host
	Pub   abstract.Point
	Peers []abstract.Point
	wg    *sync.WaitGroup
}

func (s *SimpleClient) Init(host Host, pub abstract.Point, wg *sync.WaitGroup) *SimpleClient {
	return &SimpleClient{
		Host:  host,
		Pub:   pub,
		Peers: make([]abstract.Point, 0),
		wg:    wg,
	}
}

// overridding Name host
func (s *SimpleClient) Name() string {
	return "Client " + s.Host.Name()
}

// Simplest protocol : exchange keys with the server
func (s *SimpleClient) ExchangeWithServer(name string, t *testing.T) {
	s.wg.Add(1)
	dbg.Print("ExchangeWithServer started")
	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	defer func() {
		//dbg.Print("ExchangeWithServer canceld/timed out")
		//cancel()
	}()

	// open a connection to the peer
	c := s.Open(name)
	if c == nil {
		t.Fatal("client connection is nil ><")
	}
	dbg.Print("client opened a connection to the peer")
	// create pack
	p := PublicPacket{
		Point: s.Pub,
	}
	// Send it
	err := c.Send(ctx, &p)
	if err != nil {
		t.Fatal("error sending from client:", err)
	}
	dbg.Print("Sent Public Packet")
	// Receive the response
	am, err := c.Receive(ctx)
	if err != nil {
		fmt.Printf("error receiving ..")
	}
	dbg.Print("Received response")

	// Cast to the right type
	if am.MsgType != PublicType {
		t.Fatal("Received a non-wanted packet.\n")
	}

	c.Close()
	s.wg.Done()
}

type SimpleServer struct {
	Host
	Pub abstract.Point
	t   *testing.T
	wg  *sync.WaitGroup
}

func (s *SimpleServer) Name() string {
	return "Server " + s.Host.Name()
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

	if err := c.Send(ctx, &p); err != nil {
		s.t.Fatal(err)
	}
	dbg.Print("Server sent")
	am, err := c.Receive(ctx)
	if err != nil {
		s.t.Error("Server errored when receiving packet ...\n")
	}
	dbg.Print("Server sent Public Packet")
	if am.MsgType != PublicType {
		s.t.Error("Server received a non-wanted packet\n")
	}
	dbg.Print("Closing connection")
	p = (am.Msg).(PublicPacket)
	comp := suite.Point().Base()
	dbg.Print("PublicPacket REceived from server:", p)
	if !p.Point.Equal(comp) {
		s.t.Error("point not equally reconstructed")
	}
	c.Close()
	s.wg.Done()
}

func (s *SimpleServer) Init(host Host, pub abstract.Point, t *testing.T, wg *sync.WaitGroup) *SimpleServer {
	s.Host = host
	s.Pub = pub
	s.t = t
	s.wg = wg
	return s
}

func TestTcpNetwork(t *testing.T) {
	clientHost := NewTcpHost("127.0.0.1", constructors)
	serverHost := NewTcpHost("127.0.0.1", constructors)
	clientPub := suite.Point().Base()
	serverPub := suite.Point().Add(suite.Point().Base(), suite.Point().Base())
	wg := sync.WaitGroup{}
	client := new(SimpleClient).Init(clientHost, clientPub, &wg)
	server := new(SimpleServer).Init(serverHost, serverPub, t, &wg)
	go server.Listen("127.0.0.1:5000", server.ExchangeWithClient)
	time.Sleep(1 * time.Second)
	client.ExchangeWithServer("127.0.0.1:5000", t)
	wg.Wait()
}

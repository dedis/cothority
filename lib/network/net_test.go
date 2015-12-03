package network

import (
	"bytes"
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"testing"
	"time"
)

type PublicPacket struct {
	Point abstract.Point
}

func (p *PublicPacket) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	err := Suite.Write(&b, &p.Point)
	return b.Bytes(), err
}
func (p *PublicPacket) UnmarshalBinary(buf []byte) error {
	b := bytes.NewBuffer(buf)
	err := Suite.Read(b, &p.Point)
	return err
}

var PublicType Type

func init() {
	PublicType = RegisterProtocolType(PublicPacket{})
}

type SimpleClient struct {
	Host
	Pub   abstract.Point
	Peers []abstract.Point
}

func (s *SimpleClient) Init(host Host, pub abstract.Point) *SimpleClient {
	return &SimpleClient{
		Host:  host,
		Pub:   pub,
		Peers: make([]abstract.Point, 0),
	}
}

// overridding Name host
func (s *SimpleClient) Name() string {
	return "Client " + s.Host.Name()
}

// Simplest protocol : exchange keys with the server
func (s *SimpleClient) ExchangeWithServer(name string, t *testing.T) {
	// open a connection to the peer
	c := s.Open(name)
	if c == nil {
		t.Error("client connection is nil ><")
	}
	// create pack
	p := PublicPacket{
		Point: s.Pub,
	}
	// Send it
	err := c.Send(p)
	if err != nil {
		t.Error("error sending from client:", err)
	}

	// Receive the response
	am, err := c.Receive()
	if err != nil {
		fmt.Printf("error receiving ..")
	}

	// Cast to the right type
	if am.MsgType != PublicType {
		t.Error("Received a non-wanted packet.\n")
	}

	c.Close()
}

type SimpleServer struct {
	Host
	Pub abstract.Point
	t   *testing.T
}

func (s *SimpleServer) Name() string {
	return "Server " + s.Host.Name()
}

// this is the callback when a new connection is don
func (s *SimpleServer) ExchangeWithClient(c Conn) {
	p := PublicPacket{
		Point: s.Pub,
	}

	c.Send(p)
	am, err := c.Receive()
	if err != nil {
		s.t.Error("Server errored when receiving  packet ...\n")
	}
	if am.MsgType != PublicType {
		s.t.Error("Server received a non-wanted packet\n")
	}
	c.Close()
}

func (s *SimpleServer) Init(host Host, pub abstract.Point, t *testing.T) *SimpleServer {
	s.Host = host
	s.Pub = pub
	s.t = t
	return s
}

func TestTcpNetwork(t *testing.T) {
	clientHost := NewTcpHost("127.0.0.1")
	serverHost := NewTcpHost("127.0.0.1")
	suite := edwards.NewAES128SHA256Ed25519(false)
	Suite = suite
	clientPub := suite.Point().Base()
	serverPub := suite.Point().Add(suite.Point().Base(), suite.Point().Base())
	client := new(SimpleClient).Init(clientHost, clientPub)
	server := new(SimpleServer).Init(serverHost, serverPub, t)

	go server.Listen("127.0.0.1:5000", server.ExchangeWithClient)
	time.Sleep(1 * time.Second)
	client.ExchangeWithServer("127.0.0.1:5000", t)
}

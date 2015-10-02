package main

import (
	"fmt"
	"github.com/dedis/cothority/lib/network_draft/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"os"
	"time"
)

type PublicPacket struct {
	Point abstract.Point
}

var PublicType network.Type

func init() {
	PublicType = network.RegisterProtocolType(&PublicPacket{})
}

type SimpleClient struct {
	network.Host
	Pub   abstract.Point
	Peers []abstract.Point
}

func (s *SimpleClient) Init(host network.Host, pub abstract.Point) *SimpleClient {
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
func (s *SimpleClient) ExchangeWithServer(name string) {
	fmt.Printf("Will try to connect ...\n")
	// open a connection to the peer
	c := s.Open(name)
	// create pack
	p := PublicPacket{
		Point: s.Pub,
	}
	// Send it
	c.Send(&p)
	fmt.Printf("%s has sent its PublicPacket to %s\n", s.Name(), c.PeerName())

	// Receive the response
	am, err := c.Receive()
	if err != nil {
		fmt.Printf("error receiving ..")
	}

	// Cast to the right type
	if am.MsgType != PublicType {
		fmt.Printf("Received a non-wanted packet.\n")
		os.Exit(1)
	}
	pub := am.Msg.(*PublicPacket)

	fmt.Printf("%s received the remote key from %s : %s\n", s.Name(), c.PeerName(), pub.Point.String())
	c.Close()
	fmt.Printf("%s is closing and leaving...\n", s.Name())
}

type SimpleServer struct {
	network.Host
	Pub abstract.Point
}

func (s *SimpleServer) Name() string {
	return "Server " + s.Host.Name()
}

// this is the callback when a new connection is don
func (s *SimpleServer) ExchangeWithClient(c network.Conn) {
	p := PublicPacket{
		Point: s.Pub,
	}

	c.Send(&p)
	fmt.Printf("%s has sent its PublicPacket to %s\n", s.Name(), c.PeerName())
	am, err := c.Receive()
	if err != nil {
		fmt.Printf("Server errored when receiving  packet ...\n")
	}
	if am.MsgType != PublicType {
		fmt.Printf("Server received a non-wanted packet\n")
		os.Exit(1)
	}
	pub := am.Msg.(*PublicPacket)
	fmt.Printf("%s received the remote key from %s : %s\n", s.Name(), c.PeerName(), pub.Point.String())
	c.Close()
	fmt.Printf("%s is closing ...\n", s.Name())
}

func (s *SimpleServer) Init(host network.Host, pub abstract.Point) *SimpleServer {
	s.Host = host
	s.Pub = pub
	return s
}

func main() {
	clientHost := network.NewTcpHost("127.0.0.1")
	serverHost := network.NewTcpHost("127.0.0.1")
	suite := edwards.NewAES128SHA256Ed25519(true)
	network.Suite = suite
	clientPub := suite.Point().Base()
	serverPub := suite.Point().Add(suite.Point().Base(), suite.Point().Base())
	client := new(SimpleClient).Init(clientHost, clientPub)
	server := new(SimpleServer).Init(serverHost, serverPub)

	go server.Listen(server.ExchangeWithClient)
	time.Sleep(1 * time.Second)
	client.ExchangeWithServer("127.0.0.1:" + network.ListenPort)
}

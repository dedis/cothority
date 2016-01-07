// better not get sda_test, cannot access unexported fields
package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
	"testing"
	"time"
)

var suite abstract.Suite = edwards.NewAES128SHA256Ed25519(false)

func init() {
	network.RegisterProtocolType(SimpleMessageType, SimpleMessage{})
}

// Test setting up of Host
func TestHostNew(t *testing.T) {
	Host1 := newHost("localhost:2000", suite)
	if Host1 == nil {
		t.Fatal("Couldn't setup a Host")
	}
	err := Host1.Close()
	if err != nil {
		t.Fatal("Couldn't close", err)
	}
}

// Test closing and opening of Host on same address
func TestHostClose(t *testing.T) {
	Host1 := newHost("localhost:2000", suite)
	err := Host1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	Host1 = newHost("localhost:2000", suite)
	// Needs to wait some as the listener will try for a couple of times
	// to make the connection
	time.Sleep(time.Second)
	Host1.Close()
}

// Test connection of multiple Hosts and sending messages back and forth
func TestHostMessaging(t *testing.T) {
	Host1, Host2 := setupHosts(t)
	msg := &SimpleMessage{3}
	err := Host1.SendTo("localhost:2001", msg)
	if err != nil {
		t.Fatal("Couldn't send from Host2 -> Host1:", err)
	}
	var msgDec SimpleMessage
	data, err := Host2.Receive()
	if err != nil {
		t.Fatal("Did not receive message ..")
	} else {
		if data.MsgType != sda.SDAMessageType {
			t.Fatal("Did not receive the expected type")
		}
		var ok bool
		if msgDec, ok = data.Msg.(sda.SDAMessage).Data.(SimpleMessage); !ok {
			t.Fatal("Can not convert the message")
		}
		if msgDec.I != 3 {
			t.Fatal("Received message from Host2 -> Host1 is wrong")
		}
	}
	Host1.Close()
	Host2.Close()
}

// SimpleMessage is just used to transfer one integer
type SimpleMessage struct {
	I int
}

const (
	SimpleMessageType = iota + 50
)

// Test parsing of incoming packets with regard to its double-included
// data-structure
func TestHostIncomingMessage(t *testing.T) {
	h1, h2 := setupHosts(t)
	msgSimple := &SimpleMessage{10}
	err := h1.SendTo(h2.Address, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message:", err)
	}
	am, err := h2.Receive()
	if err != nil {
		t.Fatal("Couldn't receive message", err)
	}
	dbg.Lvl3("Message received is", am)
	if am.Msg.(sda.SDAMessage).Data.(SimpleMessage).I != 10 {
		t.Fatal("Couldn't pass simple message")
	}
	h1.Close()
	h2.Close()
}

// Test propagation of peer-lists - both known and unknown
func TestPeerListPropagation(t *testing.T) {

}

// Test propagation of graphs - both known and unknown

// Test instantiation of ProtocolInstances

// Test access of actual peer that received the message
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph - ProtocolID + GraphID + InstanceID is not enough
// XXX ???

// Test complete parsing of new incoming packet
// - Test if it is SDAMessage
// - reject if unknown ProtocolID
// - setting up of graph and Hostlist
// - instantiating ProtocolInstance

// privPub creates a private/public key pair
func privPub(s abstract.Suite) (abstract.Secret, abstract.Point) {
	keypair := &config.KeyPair{}
	keypair.Gen(s, random.Stream)
	return keypair.Secret, keypair.Public
}

func newHost(address string, s abstract.Suite) *sda.Host {
	priv, _ := privPub(s)
	return sda.NewHost(address, s, priv, network.NewTcpHost(network.DefaultConstructors(s)))
}

func setupHosts(t *testing.T) (*sda.Host, *sda.Host) {
	dbg.TestOutput(testing.Verbose(), 4)
	Host1 := newHost("localhost:2000", suite)
	// make the second peer as the server
	Host2 := newHost("localhost:2001", suite)
	// make it listen
	Host2.Listen()
	_, err := Host1.Connect(Host2.Address)
	if err != nil {
		t.Fatal(err)
	}
	return Host1, Host2
}

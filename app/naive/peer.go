package main

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	net "github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"golang.org/x/net/context"
)

// Impl of the "naive sign" protocol
// i.e. leader collects every signature from every other peers

const ServRole string = "server"
const LeadRole string = "leader"

const msgMaxLenght int = 256

var suite abstract.Suite

type BasicSignature struct {
	Pub   abstract.Point
	Chall abstract.Secret
	Resp  abstract.Secret
}

type MessageSigning struct {
	Length int
	Msg    []byte
}

const (
	BasicSignatureType = iota + 222
	MessageSigningType
)

func init() {
	net.RegisterProtocolType(BasicSignatureType, BasicSignature{})
	net.RegisterProtocolType(MessageSigningType, MessageSigning{})
}

// Set up some global variables such as the different messages used during
// this protocol and the general suite to be used
func init() {
	suite = edwards.NewAES128SHA256Ed25519(false)
}

// the struct representing the role of leader
type Peer struct {
	net.Host

	// the longterm key of the peer
	priv abstract.Secret
	pub  abstract.Point

	// role is server or leader
	role string

	// leader part
	Conns      []net.Conn
	Pubs       []abstract.Point
	Signatures []BasicSignature

	Name string
}

func (l *Peer) String() string {
	return fmt.Sprintf("%s %s", l.Name, l.role)
}

// Will send the message to be signed to everyone
func (l *Peer) SendMessage(msg []byte, c net.Conn) {
	if len(msg) > msgMaxLenght {
		dbg.Fatal("Tried to send a too big message to sign. Abort")
	}
	ms := new(MessageSigning)
	ms.Length = len(msg)
	ms.Msg = msg
	ctx := context.TODO()
	err := c.Send(ctx, ms)
	if err != nil {
		dbg.Fatal("Could not send message to", c.Remote())
	}
}

// Wait for the leader to receive the generated signatures from the servers
func (l *Peer) ReceiveBasicSignature(c net.Conn) *BasicSignature {
	ctx := context.TODO()
	appMsg, err := c.Receive(ctx)
	if err != nil {
		dbg.Fatal(l.String(), "error decoding message from", c.Remote())
	}
	if appMsg.MsgType != BasicSignatureType {
		dbg.Fatal(l.String(), "Received an unknown type:", appMsg.MsgType.String())
	}
	bs := appMsg.Msg.(BasicSignature)
	return &bs
}

func (l *Peer) Signature(msg []byte) *BasicSignature {
	rand := suite.Cipher([]byte("cipher"))

	sign := SchnorrSign(suite, rand, msg, l.priv)
	sign.Pub = l.pub
	return &sign
}

func NewPeer(host net.Host, name, role string, secret abstract.Secret,
	public abstract.Point) *Peer {
	return &Peer{
		role: role,
		Host: host,
		priv: secret,
		pub:  public,
		Name: name,
	}
}

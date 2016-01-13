package main

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	net "github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"golang.org/x/net/context"
)

const msgMaxLenght int = 256

// Treee terminology
const LeadRole = "root"
const ServRole = "node"

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

type ListBasicSignature struct {
	Length int
	Sigs   []BasicSignature
}

const (
	BasicSignatureType = iota + 222
	MessageSigningType
	ListBasicSignatureType
)

func init() {
	net.RegisterProtocolType(BasicSignatureType, BasicSignature{})
	net.RegisterProtocolType(MessageSigningType, MessageSigning{})
	net.RegisterProtocolType(ListBasicSignatureType, ListBasicSignature{})
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
	Name       string
}

func (l *Peer) String() string {
	return fmt.Sprintf("%s (%s)", l.Name, l.role)
}

func (l *Peer) Signature(msg []byte) *BasicSignature {
	rand := suite.Cipher([]byte("cipher"))

	sign := SchnorrSign(suite, rand, msg, l.priv)
	sign.Pub = l.pub
	return &sign
}

func (l *Peer) ReceiveMessage(c net.Conn) MessageSigning {
	ctx := context.TODO()
	app, err := c.Receive(ctx)
	if err != nil {
		dbg.Fatal(l.String(), "could not receive message from", c.Remote())

	}
	if app.MsgType != MessageSigningType {
		dbg.Fatal(l.String(), "MS error: received", app.MsgType.String(), "from", c.Remote())
	}
	return app.Msg.(MessageSigning)
}

func (l *Peer) ReceiveListBasicSignature(c net.Conn) ListBasicSignature {
	ctx := context.TODO()
	app, err := c.Receive(ctx)
	if err != nil {
		dbg.Fatal(l.String(), "could not receive listbasicsig from", c.Remote())
	}

	if app.MsgType != ListBasicSignatureType {
		dbg.Fatal(l.String(), "LBS error: received", app.MsgType.String(), "from", c.Remote())
	}
	return app.Msg.(ListBasicSignature)

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

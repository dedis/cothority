package main

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	net "github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
)

const msgMaxLenght int = 256

// Treee terminology
const LeadRole = "root"
const ServRole = "node"

var suite abstract.Suite

// Set up some global variables such as the different messages used during
// this protocol and the general suite to be used
func init() {
	suite = edwards.NewAES128SHA256Ed25519(false)
	net.Suite = suite
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
	Signatures []net.BasicSignature
}

func (l *Peer) String() string {
	return fmt.Sprintf("%s (%s)", l.Host.Name(), l.role)
}

func (l *Peer) Signature(msg []byte) *net.BasicSignature {
	rand := suite.Cipher([]byte("cipher"))

	sign := SchnorrSign(suite, rand, msg, l.priv)
	sign.Pub = l.pub
	return &sign
}

func (l *Peer) ReceiveMessage(c net.Conn) net.MessageSigning {
	app, err := c.Receive()
	if err != nil {
		dbg.Fatal(l.String(), "could not receive message from ", c.PeerName())

	}
	if app.MsgType != net.MessageSigningType {
		dbg.Fatal(l.String(), "MS error : received ", app.MsgType.String(), " from ", c.PeerName())
	}
	return app.Msg.(net.MessageSigning)
}

func (l *Peer) ReceiveListBasicSignature(c net.Conn) net.ListBasicSignature {
	app, err := c.Receive()
	if err != nil {
		dbg.Fatal(l.String(), "could not receive listbasicsig from ", c.PeerName())
	}

	if app.MsgType != net.ListBasicSignatureType {
		dbg.Fatal(l.String(), "LBS error : received ", app.MsgType.String(), "from ", c.PeerName())
	}
	return app.Msg.(net.ListBasicSignature)

}
func NewPeer(host net.Host, role string, secret abstract.Secret,
	public abstract.Point) *Peer {
	return &Peer{
		role: role,
		Host: host,
		priv: secret,
		pub:  public,
	}
}

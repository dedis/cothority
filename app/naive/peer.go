package main

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	net "github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
)

// Impl of the "naive sign" protocol
// i.e. leader collects every signature from every other peers

const ServRole string = "server"
const LeadRole string = "leader"

const msgMaxLenght int = 256

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
	return fmt.Sprintf("%s %s", l.Host.Name(), l.role)
}

// Will send the message to be signed to everyone
func (l *Peer) SendMessage(msg []byte, c net.Conn) {
	if len(msg) > msgMaxLenght {
		dbg.Fatal("Tried to send a too big message to sign. Abort")
	}
	ms := new(net.MessageSigning)
	ms.Length = len(msg)
	ms.Msg = msg
	err := c.Send(*ms)
	if err != nil {
		dbg.Fatal("Could not send message to ", c.PeerName())
	}
}

// Wait for the leader to receive the generated signatures from the servers
func (l *Peer) ReceiveBasicSignature(c net.Conn) *net.BasicSignature {

	appMsg, err := c.Receive()
	if err != nil {
		dbg.Fatal(l.String(), "error decoding message from ", c.PeerName())
	}
	if appMsg.MsgType != net.BasicSignatureType {
		dbg.Fatal(l.String(), "Received an unknown type : ", appMsg.MsgType.String())
	}
	bs := appMsg.Msg.(net.BasicSignature)
	return &bs
}

func (l *Peer) Signature(msg []byte) *net.BasicSignature {
	rand := suite.Cipher([]byte("cipher"))

	sign := SchnorrSign(suite, rand, msg, l.priv)
	sign.Pub = l.pub
	return &sign
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

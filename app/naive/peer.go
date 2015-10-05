package main

import (
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/network_draft/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
)

// Impl of the "naive sign" protocol
// i.e. leader collects every signature from every other peers

const ServRole string = "server"
const LeadRole string = "leader"

const msgMaxLenght int = 256

var suite abstract.Suite

var MessageSigningType network.Type
var BasicSignatureType network.Type

// Set up some global variables such as the different messages used during
// this protocol and the general suite to be used
func init() {
	suite = edwards.NewAES128SHA256Ed25519(true)
	network.Suite = suite
	MessageSigningType = network.RegisterProtocolType(MessageSigning{})
	BasicSignatureType = network.RegisterProtocolType(BasicSignature{})
}

// the struct representing the role of leader
type Peer struct {
	network.Host

	// the longterm key of the peer
	priv abstract.Secret
	pub  abstract.Point

	// role is server or leader
	role string

	// leader part
	Conns      []network.Conn
	Pubs       []abstract.Point
	Signatures []BasicSignature
}

// Message used to transmit our generated signature
type BasicSignature struct {
	Pub   abstract.Point
	Chall abstract.Secret
	Resp  abstract.Secret
}

// The message used to transmit our message to be signed
// TODO modify network so we can build packet that will be
// decoded with our own marshalbinary /unmarshal. We could
// then pass the size of the msg to be read before the msg
// itself.
type MessageSigning struct {
	Msg [msgMaxLenght]byte
}

func (l *Peer) String() string {
	return fmt.Sprintf("%s %s (%d sigs)", l.Host.Name(), l.role, len(l.Signatures))
}

// Will send the message to be signed to everyone
func (l *Peer) SendMessage(msg []byte, c network.Conn) {
	if len(msg) > msgMaxLenght {
		dbg.Fatal("Tried to send a too big message to sign. Abort")
	}
	ms := new(MessageSigning)
	copy(ms.Msg[:], msg)

	err := c.Send(ms)
	if err != nil {
		dbg.Fatal("Could not send message to ", c.PeerName())
	}
}

// Wait for the leader to receive the generated signatures from the servers
func (l *Peer) ReceiveBasicSignature(c network.Conn) BasicSignature {

	appMsg, err := c.Receive()
	if err != nil {
		dbg.Fatal(l.String(), "error decoding message from ", c.PeerName())
	}
	if appMsg.MsgType != BasicSignatureType {
		dbg.Fatal(l.String(), "Received an unknown type : ", appMsg.MsgType.String())
	}
	bs := appMsg.Msg.(BasicSignature)
	return bs
}

func (l *Peer) Signature(msg []byte) BasicSignature {
	rand := suite.Cipher([]byte("cipher"))
	x := suite.Secret().Pick(rand)

	sign := SchnorrSign(suite, rand, msg, x)
	sign.Pub = l.pub
	return sign
}

func NewPeer(host network.Host, role string, secret abstract.Secret,
	public abstract.Point) *Peer {
	return &Peer{
		role: role,
		Host: host,
		priv: secret,
		pub:  public,
	}
}

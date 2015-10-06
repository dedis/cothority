package main

import (
	"bytes"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/network_draft/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"io"
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
	Length int
	Msg    []byte
}

// Messagesigning must implement Marshaling interface
// so it can decode any variable length message
func (m *MessageSigning) String() string {
	return "MessageSigning " + fmt.Sprintf("%d", m.Length)
}
func (m *MessageSigning) MarshalSize() int {
	var b bytes.Buffer
	_ = suite.Write(&b, m.Length)
	return len(m.Msg) + len(b.Bytes())
}
func (m *MessageSigning) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	err := suite.Write(&b, m.Length, m.Msg)
	by := b.Bytes()
	return by, err
}

func (m *MessageSigning) MarshalTo(w io.Writer) (int, error) {
	err := suite.Write(w, m.Length, m.Msg)
	return 0, err
}

// TODO really considerate wether UnmarshalFrom should return int or no..
// abstract encoding does not use it, nor does it use MarshalSIze...
func (m *MessageSigning) UnmarshalFrom(r io.Reader) (int, error) {
	err := suite.Read(r, &m.Length)
	if err != nil {
		return 0, err
	}
	m.Msg = make([]byte, m.Length)
	err = suite.Read(r, m.Msg)
	return 0, err
}

func (m *MessageSigning) UnmarshalBinary(buf []byte) error {
	b := bytes.NewBuffer(buf)
	_, err := m.UnmarshalFrom(b)
	return err
}
func (l *Peer) String() string {
	return fmt.Sprintf("%s %s", l.Host.Name(), l.role)
}

// Will send the message to be signed to everyone
func (l *Peer) SendMessage(msg []byte, c network.Conn) {
	if len(msg) > msgMaxLenght {
		dbg.Fatal("Tried to send a too big message to sign. Abort")
	}
	ms := new(MessageSigning)
	ms.Length = len(msg)
	ms.Msg = msg
	err := c.Send(*ms)
	if err != nil {
		dbg.Fatal("Could not send message to ", c.PeerName())
	}
}

// Wait for the leader to receive the generated signatures from the servers
func (l *Peer) ReceiveBasicSignature(c network.Conn) *BasicSignature {

	appMsg, err := c.Receive()
	if err != nil {
		dbg.Fatal(l.String(), "error decoding message from ", c.PeerName())
	}
	if appMsg.MsgType != BasicSignatureType {
		dbg.Fatal(l.String(), "Received an unknown type : ", appMsg.MsgType.String())
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

func NewPeer(host network.Host, role string, secret abstract.Secret,
	public abstract.Point) *Peer {
	return &Peer{
		role: role,
		Host: host,
		priv: secret,
		pub:  public,
	}
}

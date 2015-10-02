package main

import (
	"github.com/dedis/cothority/lib/network_draft/network"
)

// Impl of the "naive sign" protocol
// i.e. leader collects every signature from every other peers

const msgMaxLenght int = 256

const suite = edwards.NewAES128SHA256Ed25519(true)

var MessageSigningType network.Type
var BasicSignature network.Type

func init() {
	network.Suite = suite
	MessageSigningType = network.RegisterProtocolType(MessageSigning{})
	BasicSignature = network.RegisterProtocolType(BasicSignature{})
}

type Leader struct {
	network.Host
	Conns      []network.Conn
	Pubs       []abstract.Point
	Signatures []BasicSignature
}

type Server struct {
	network.Host

	Pub  abstract.Point
	done bool
}

type PublicKey struct {
	Pub abstract.Point
}

type BasicSignature struct {
	Chall abstract.Secret
	Resp  abstract.Secret
}

type MessageSigning struct {
	msg [msgMaxLenght]byte
}

func (l *Leader) String() {
	return l.Host.Name() + fmt.Sprintf(" Leader (%d sigs)", len(l.Signatures))
}

func (s *Server) String() {
	return s.Host.Name() + fmt.Sprintf(" Server (done %v)", s.done)
}

func (l *Leader) SendMessage(msg []byte) {
	if len(msg) > msgMaxLenght {
		dbg.Fatal("Tried to send a too big message to sign. Abort")
	}
	ms := new(messageSigning)
	copy(ms.msg, msg)

	for c := range l.Conns {
		err := c.Send(ms)
		if err != nil {
			dbg.Fatal("Could not send message to ", c.PeerName())
		}
	}
}

func (l *Leader) ReceiveBasicSignatures() {
	ch := make(chan BasicSignature)

	for c := range l.Conns {
		go func(con network.Conn) {
			appMsg, err := con.Receive()
			if err != nil {
				dbg.Fatal(l.String(), "error decoding message from ", c.PeerName())
			}
			if appMsg.MsgType != BasicSignatureType {
				dbg.Fatal(l.String(), "Received an unknown type : ", app.MsgType.String())
			}
			bs := appMsg.Msg.(BasicSignature)
			ch <- bs
		}()
	}
	dbg.Lvl2(l.String(), "Waiting on basic signatures from servers ...")
	for {
		bs := <-ch
		l.Signatures = append(l.Signatures, bs)
		if len(l.Signatures) == len(l.Conns) {
			break
		}
	}
	dbg.Lvl2(l.String(), "Received all signatures..")
}

func (l *Leader) VerifySignatures(msg []byte) {

}

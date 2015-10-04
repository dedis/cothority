package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/network_draft/network"
	"strings"
)

func RunServer(conf *app.NaiveConfig) {
	dbg.Lvl3("Launching a naiv_sign.")
	indexPeer := -1
	for i, h := range conf.Hosts {
		if h == app.RunFlags.Hostname {
			indexPeer = i
		}
	}
	if indexPeer == -1 {
		dbg.Fatal("Could not find its own hostname. Abort")
	}

	if indexPeer == 0 {
		GoLeader(conf)
	} else {
		GoServer(conf)
	}

}

func GoLeader(conf *app.NaiveConfig) {
	// TODO remove this dirty fix and make
	// ... a clean one.
	ip := strings.Split(app.RunFlags.Hostname, ":")[0]

	host := network.NewTcpHost(ip)
	key := cliutils.KeyPair(suite)
	leader := NewPeer(host, LeadRole, key.Secret, key.Public)

	msg := []byte("Hello World\n")
	// Listen for connections
	dbg.Lvl1("Leader making connections ...")
	connChan := make(chan BasicSignature)
	// Send the message to be signed
	proto := func(c network.Conn) {
		leader.SendMessage(msg, c)
		sig := leader.ReceiveBasicSignature(c)
		c.Close()
		connChan <- sig

	}
	go leader.Listen(proto)
	dbg.Lvl1("Leader Listening for signatures..")
	n := 0
	faulty := 0
	// verify each coming signatures
	for n < len(conf.Hosts)-1 {
		bs := <-connChan
		dbg.Lvl2("Leader received signature")
		if err := SchnorrVerify(suite, msg, bs); err != nil {
			faulty += 1
			dbg.Lvl2("Leader received a faulty signature !")
		}
		n += 1
	}
	dbg.Lvl1("Leader received ", len(conf.Hosts)-1, "signatures (", faulty, " faulty sign)")
}

func GoServer(conf *app.NaiveConfig) {
	// TODO remove this dirty fix and make
	// ... a clean one.
	ip := strings.Split(app.RunFlags.Hostname, ":")[0]

	host := network.NewTcpHost(ip)
	key := cliutils.KeyPair(suite)
	server := NewPeer(host, LeadRole, key.Secret, key.Public)
	dbg.Lvl1("Server will contact leader .")
	l := server.Open(conf.Hosts[0])
	dbg.Lvl1("Server is connected")
	m, err := l.Receive()
	if err != nil {
		dbg.Fatal("server received error waiting msg")
	}
	if m.MsgType != MessageSigningType {
		dbg.Fatal("Server wanted to receive a msg to sign but..", m.MsgType.String())
	}
	msg := m.Msg.(*MessageSigning).Msg

	s := server.Signature(msg[:])
	l.Send(s)
	l.Close()
	dbg.Lvl1("Server sent signature.Fin")

}

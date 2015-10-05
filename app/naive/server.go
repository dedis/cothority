package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/network_draft/network"
	"strings"
)

func RunServer(conf *app.NaiveConfig) {
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
		dbg.Lvl3("Launching a naiv_sign. : Leader ", app.RunFlags.Hostname)
		GoLeader(conf)
	} else {
		dbg.Lvl3("Launching a naiv_sign : Server ", app.RunFlags.Hostname)
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
	dbg.Lvl1(app.RunFlags.Hostname, "Leader making connections ...")
	connChan := make(chan *BasicSignature)
	// Send the message to be signed
	proto := func(c network.Conn) {
		dbg.Lvl3(leader.String(), "sending message ", msg, "to server ", c.PeerName())
		leader.SendMessage(msg, c)
		dbg.Lvl3(leader.String(), "receivng signature from server", c.PeerName())
		sig := leader.ReceiveBasicSignature(c)
		c.Close()
		dbg.Lvl3(leader.String(), "closed connection with server", c.PeerName())
		connChan <- sig
	}

	go leader.Listen(app.RunFlags.Hostname, proto)
	dbg.Lvl1(app.RunFlags.Hostname, "Leader Listening for signatures..")
	n := 0
	faulty := 0
	// verify each coming signatures
	for n < len(conf.Hosts)-1 {
		bs := <-connChan
		dbg.Lvl2(app.RunFlags.Hostname, "Leader received signature")
		if err := SchnorrVerify(suite, msg, *bs); err != nil {
			faulty += 1
			dbg.Lvl2(app.RunFlags.Hostname, "Leader received a faulty signature !")
		}
		n += 1
	}
	dbg.Lvl1(app.RunFlags.Hostname, "Leader received ", len(conf.Hosts)-1, "signatures (", faulty, " faulty sign)")
}

func GoServer(conf *app.NaiveConfig) {
	host := network.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)
	server := NewPeer(host, ServRole, key.Secret, key.Public)
	dbg.Lvl2(server.String(), "Server will contact leader .")
	l := server.Open(conf.Hosts[0])
	dbg.Lvl1(server.String(), "Server is connected to leader ", l.PeerName())
	m, err := l.Receive()
	dbg.Lvl2(server.String(), "received the message to be signed from the leader")
	if err != nil {
		dbg.Fatal(server.String(), "server received error waiting msg")
	}
	if m.MsgType != MessageSigningType {
		dbg.Fatal(app.RunFlags.Hostname, "Server wanted to receive a msg to sign but..", m.MsgType.String())
	}
	msg := m.Msg.(MessageSigning).Msg
	dbg.Lvl3(server.String(), "received msg : ", msg[:])
	s := server.Signature(msg[:])
	dbg.Lvl2(server.String(), "will send the signature to leader")
	l.Send(*s)
	l.Close()
	dbg.Lvl1(app.RunFlags.Hostname, "Server sent signature.Fin")

}

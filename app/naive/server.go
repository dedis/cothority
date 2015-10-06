package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/lib/network_draft/network"
	"time"
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
		dbg.Lvl2("Launching a naiv_sign. : Leader ", app.RunFlags.Hostname)
		GoLeader(conf)
	} else {
		dbg.Lvl2("Launching a naiv_sign : Server ", app.RunFlags.Hostname)
		GoServer(conf)
	}

}

func GoLeader(conf *app.NaiveConfig) {

	host := network.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)
	leader := NewPeer(host, LeadRole, key.Secret, key.Public)

	msg := []byte("Hello World\n")
	// Listen for connections
	dbg.Lvl1(leader.String(), "making connections ...")
	// each conn will create its own channel to be used to handle rounds
	roundChans := make(chan chan chan *BasicSignature)
	// Send the message to be signed
	proto := func(c network.Conn) {
		// make the chan that will receive a new chan
		// for each round where to send the signature
		roundChan := make(chan chan *BasicSignature)
		roundChans <- roundChan
		n := 0
		// wait for the next round
		for sigChan := range roundChan {
			dbg.Lvl3(leader.String(), "Round ", n, " sending message ", msg, "to server ", c.PeerName())
			leader.SendMessage(msg, c)
			dbg.Lvl3(leader.String(), "Round ", n, " receivng signature from server", c.PeerName())
			sig := leader.ReceiveBasicSignature(c)
			sigChan <- sig
			n += 1
		}
		c.Close()
		dbg.Lvl3(leader.String(), "closed connection with server", c.PeerName())
	}
	now := time.Now()
	go leader.Listen(app.RunFlags.Hostname, proto)
	dbg.Lvl2(leader.String(), "Listening for channels creation..")
	// listen for round chans + signatures for each round
	masterRoundChan := make(chan chan *BasicSignature)
	roundChanns := make([]chan chan *BasicSignature, 0)
	//  Make the "setup" of channels
	for {
		ch := <-roundChans
		roundChanns = append(roundChanns, ch)
		//Received round channels from every connections-
		if len(roundChanns) == len(conf.Hosts)-1 {
			// make the Fanout => master will send to all
			go func() {
				// send the new SignatureChannel to every conn
				for newSigChan := range masterRoundChan {
					for _, c := range roundChanns {
						c <- newSigChan
					}
				}
				//close when finished
				for _, c := range roundChanns {
					close(c)
				}
			}()
			break
		}
	}
	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": "naive_setup",
		"time": time.Since(now)}).Info("")
	dbg.Lvl1(leader.String(), "got all channels ready => starting the ", conf.Rounds, " rounds")
	for i := 0; i < conf.Rounds; i++ {
		now = time.Now()
		n := 0
		faulty := 0
		// launch a new round
		connChan := make(chan *BasicSignature)
		masterRoundChan <- connChan
		// verify each coming signatures
		for n < len(conf.Hosts)-1 {
			bs := <-connChan
			if err := SchnorrVerify(suite, msg, *bs); err != nil {
				faulty += 1
				dbg.Lvl2(leader.String(), "Round ", i, " received a faulty signature !")
			} else {
				dbg.Lvl2(leader.String(), "Round ", i, " received Good signature")
			}
			n += 1
		}
		dbg.Lvl1(leader.String(), "Round ", i, " received ", len(conf.Hosts)-1, "signatures (", faulty, " faulty sign)")
		log.WithFields(log.Fields{
			"file":  logutils.File(),
			"type":  "naive_round",
			"round": i,
			"time":  time.Since(now),
		}).Info("")
	}
	close(masterRoundChan)
	dbg.Lvl1(leader.String(), " Has done all rounds")
	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": "end"}).Info("")
}

func GoServer(conf *app.NaiveConfig) {
	time.Sleep(2 * time.Second)
	host := network.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)
	server := NewPeer(host, ServRole, key.Secret, key.Public)
	dbg.Lvl2(server.String(), "will contact leader ", conf.Hosts[0])
	l := server.Open(conf.Hosts[0])
	dbg.Lvl1(server.String(), "is connected to leader ", l.PeerName())

	// make the protocol for each rounds
	for i := 0; i < conf.Rounds; i++ {
		// Receive message
		m, err := l.Receive()
		dbg.Lvl2(server.String(), " round ", i, " received the message to be signed from the leader")
		if err != nil {
			dbg.Fatal(server.String(), "round ", i, " received error waiting msg")
		}
		if m.MsgType != MessageSigningType {
			dbg.Fatal(app.RunFlags.Hostname, "round ", i, "  wanted to receive a msg to sign but..", m.MsgType.String())
		}
		msg := m.Msg.(MessageSigning).Msg
		dbg.Lvl3(server.String(), "round ", i, " received msg : ", msg[:])
		// Gen signature & send
		s := server.Signature(msg[:])
		l.Send(*s)
		dbg.Lvl2(server.String(), "round ", i, " sent the signature to leader")
	}
	l.Close()
	dbg.Lvl1(app.RunFlags.Hostname, "Finished")

}

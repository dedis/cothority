package main

/*
 * This is a simple (naive) implementation of a multi-signature protocol
 * where the *leader* sends the message to every *signer* who signs it and
 * returns the result to the server.
 */

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	net "github.com/dedis/cothority/lib/network_draft/network"
	"sync"
	"time"
)

// Searches for the index in the hostlist and decides if we're the leader
// or one of the clients
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
		dbg.Lvl2("Launching a naiv_sign : Signer ", app.RunFlags.Hostname)
		GoSigner(conf)
	}
}

// This is the leader who waits for all connections and then sends the
// message to be signed
func GoLeader(conf *app.NaiveConfig) {

	host := net.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)
	leader := NewPeer(host, LeadRole, key.Secret, key.Public)

	// Setting up the connections

	msg := []byte("Hello World\n")
	// Listen for connections
	dbg.Lvl2(leader.String(), "making connections ...")
	// each conn will create its own channel to be used to handle rounds
	roundChans := make(chan chan chan *net.BasicSignature)
	// Send the message to be signed
	proto := func(c net.Conn) {
		// make the chan that will receive a new chan
		// for each round where to send the signature
		roundChan := make(chan chan *net.BasicSignature)
		roundChans <- roundChan
		n := 0
		// wait for the next round
		for sigChan := range roundChan {
			dbg.Lvl3(leader.String(), "Round ", n, " sending message ", msg, "to signer ", c.PeerName())
			leader.SendMessage(msg, c)
			dbg.Lvl3(leader.String(), "Round ", n, " receivng signature from signer", c.PeerName())
			sig := leader.ReceiveBasicSignature(c)
			sigChan <- sig
			n += 1
		}
		c.Close()
		dbg.Lvl3(leader.String(), "closed connection with signer", c.PeerName())
	}

	// Connecting to the signer

	now := time.Now()
	go leader.Listen(app.RunFlags.Hostname, proto)
	dbg.Lvl2(leader.String(), "Listening for channels creation..")
	// listen for round chans + signatures for each round
	masterRoundChan := make(chan chan *net.BasicSignature)
	roundChanns := make([]chan chan *net.BasicSignature, 0)
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
					for i, _ := range roundChanns {
						go func(j int) { roundChanns[j] <- newSigChan }(i)
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
	dbg.Lvl2(leader.String(), "got all channels ready => starting the ", conf.Rounds, " rounds")

	// Starting to run the simulation for conf.Rounds rounds

	for round := 0; round < conf.Rounds; round++ {
		now = time.Now()
		n := 0
		faulty := 0
		// launch a new round
		connChan := make(chan *net.BasicSignature)
		masterRoundChan <- connChan
		var wg sync.WaitGroup
		wg.Add(len(conf.Hosts) - 1)
		// verify each coming signatures
		for n < len(conf.Hosts)-1 {
			bs := <-connChan
			//			go func(b *net.BasicSignature) {
			//				if err := SchnorrVerify(suite, msg, *b); err != nil {
			//					faulty += 1
			//					dbg.Lvl2(leader.String(), "Round ", i, " received a faulty signature !")
			//				} else {
			//					dbg.Lvl2(leader.String(), "Round ", i, " received Good signature")
			//				}
			if conf.SkipChecks {
				dbg.Lvl2("Skipping check for round", round)
				wg.Done()
			} else {
				go func(b *BasicSignature) {
					if err := SchnorrVerify(suite, msg, *b); err != nil {
						faulty += 1
						dbg.Lvl2(leader.String(), "Round ", round, " received a faulty signature !")
					} else {
						dbg.Lvl2(leader.String(), "Round ", round, " received Good signature")
					}
					wg.Done()
				}(bs)
			}
			n += 1
		}
		wg.Wait()
		dbg.Lvl2(leader.String(), "Round ", round, " received ", len(conf.Hosts)-1, "signatures (",
			faulty, " faulty sign)")
		log.WithFields(log.Fields{
			"file":  logutils.File(),
			"type":  "naive_round",
			"round": round,
			"time":  time.Since(now),
		}).Info("")
	}

	// Close down all connections

	close(masterRoundChan)
	dbg.Lvl2(leader.String(), " Has done all rounds")
	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": "end"}).Info("")
}

// The signer connects to the leader and then waits for a message to be
// signed
func GoSigner(conf *app.NaiveConfig) {
	// Wait for leader to be ready
	time.Sleep(2 * time.Second)
	host := net.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)
	signer := NewPeer(host, ServRole, key.Secret, key.Public)
	dbg.Lvl2(signer.String(), "will contact leader ", conf.Hosts[0])
	l := signer.Open(conf.Hosts[0])
	dbg.Lvl2(signer.String(), "is connected to leader ", l.PeerName())

	// make the protocol for each round
	for round := 0; round < conf.Rounds; round++ {
		// Receive message
		m, err := l.Receive()
		dbg.Lvl2(signer.String(), " round ", round, " received the message to be signed from the leader")
		if err != nil {
			dbg.Fatal(signer.String(), "round ", round, " received error waiting msg")
		}
		if m.MsgType != MessageSigningType {
			dbg.Fatal(app.RunFlags.Hostname, "round ", round, "  wanted to receive a msg to sign but..",
				m.MsgType.String())
		}
		msg := m.Msg.(MessageSigning).Msg
		dbg.Lvl3(signer.String(), "round ", round, " received msg : ", msg[:])
		// Generate signature & send
		s := signer.Signature(msg[:])
		l.Send(*s)
		dbg.Lvl2(signer.String(), "round ", round, " sent the signature to leader")
	}
	l.Close()
	dbg.Lvl2(app.RunFlags.Hostname, "Finished")

}

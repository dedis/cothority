package main

/*
 * This is a simple (naive) implementation of a multi-signature protocol
 * where the *leader* sends the message to every *signer* who signs it and
 * returns the result to the server.
 */

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	net "github.com/dedis/cothority/lib/network"
	"golang.org/x/net/context"
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
		dbg.Lvl3("Launching a naiv_sign.: Leader", app.RunFlags.Hostname)
		GoLeader(conf)
		monitor.End()
	} else {
		dbg.Lvl3("Launching a naiv_sign: Signer", app.RunFlags.Hostname)
		GoSigner(conf)
	}
}

// This is the leader who waits for all connections and then sends the
// message to be signed
func GoLeader(conf *app.NaiveConfig) {

	host := net.NewTcpHost()
	key := cliutils.KeyPair(suite)
	leader := NewPeer(host, app.RunFlags.Hostname, LeadRole, key.Secret, key.Public)

	// Setting up the connections
	// notably to the monitoring process
	if app.RunFlags.Monitor != "" {
		monitor.ConnectSink(app.RunFlags.Monitor)
	} else {
		monitor.EnableMeasure(false)
	}
	msg := []byte("Hello World\n")
	// Listen for connections
	dbg.Lvl3(leader.String(), "making connections ...")
	// each conn will create its own channel to be used to handle rounds
	roundChans := make(chan chan chan *BasicSignature)
	// Send the message to be signed
	proto := func(c net.Conn) {
		// make the chan that will receive a new chan
		// for each round where to send the signature
		roundChan := make(chan chan *BasicSignature)
		roundChans <- roundChan
		n := 0
		// wait for the next round
		for sigChan := range roundChan {
			dbg.Lvl3(leader.String(), "Round", n, "sending message", msg, "to signer", c.Remote())
			leader.SendMessage(msg, c)
			dbg.Lvl3(leader.String(), "Round", n, "receivng signature from signer", c.Remote())
			sig := leader.ReceiveBasicSignature(c)
			sigChan <- sig
			n += 1
		}
		c.Close()
		dbg.Lvl3(leader.String(), "closed connection with signer", c.Remote())
	}

	// Connecting to the signer
	setup := monitor.NewMeasure("setup")
	go leader.Listen(app.RunFlags.Hostname, proto)
	dbg.Lvl3(leader.String(), "Listening for channels creation..")
	// listen for round chans + signatures for each round
	masterRoundChan := make(chan chan *BasicSignature)
	roundChanns := make([]chan chan *BasicSignature, 0)
	numberHosts := len(conf.Hosts)
	//  Make the "setup" of channels
	for {
		ch := <-roundChans
		roundChanns = append(roundChanns, ch)
		//Received round channels from every connections-
		if len(roundChanns) == numberHosts-1 {
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
	setup.Measure()
	dbg.Lvl3(leader.String(), "got all channels ready => starting the", conf.Rounds, "rounds")

	// Starting to run the simulation for conf.Rounds rounds

	roundM := monitor.NewMeasure("round")
	for round := 0; round < conf.Rounds; round++ {
		// Measure calculation time
		calc := monitor.NewMeasure("calc")
		dbg.Lvl1("Server starting round", round+1)
		n := 0
		faulty := 0
		// launch a new round
		connChan := make(chan *BasicSignature)
		masterRoundChan <- connChan

		// Wait each signatures
		sigs := make([]*BasicSignature, 0)
		for n < numberHosts-1 {
			bs := <-connChan
			sigs = append(sigs, bs)
			n += 1
		}
		// All sigs reeived <=> all calcs are done
		calc.Measure()

		// verify each signatures
		if conf.SkipChecks {
			dbg.Lvl3("Skipping check for round", round)
		} else {
			// Measure verificationt time
			verify := monitor.NewMeasure("verify")
			for _, sig := range sigs {
				if err := SchnorrVerify(suite, msg, *sig); err != nil {
					faulty += 1
					dbg.Lvl1(leader.String(), "Round", round, "received a faulty signature!")
				} else {
					dbg.Lvl3(leader.String(), "Round", round, "received Good signature")
				}
			}
			verify.Measure()
		}
		roundM.Measure()
		dbg.Lvl3(leader.String(), "Round", round, "received", len(conf.Hosts)-1, "signatures (",
			faulty, "faulty sign)")
	}

	// Close down all connections
	close(masterRoundChan)
	dbg.Lvl3(leader.String(), "has done all rounds")
}

// The signer connects to the leader and then waits for a message to be
// signed
func GoSigner(conf *app.NaiveConfig) {
	// Wait for leader to be ready
	time.Sleep(2 * time.Second)
	host := net.NewTcpHost()
	key := cliutils.KeyPair(suite)
	signer := NewPeer(host, app.RunFlags.Hostname, ServRole, key.Secret, key.Public)
	dbg.Lvl3(signer.String(), "will contact leader", conf.Hosts[0])
	l, err := signer.Open(conf.Hosts[0])
	if err != nil {
		dbg.Fatal("Could not open connection to", conf.Hosts[0])
	}
	dbg.Lvl3(signer.String(), "is connected to leader", l.Remote())

	// make the protocol for each round
	for round := 0; round < conf.Rounds; round++ {
		// Receive message
		ctx := context.TODO()
		m, err := l.Receive(ctx)
		dbg.Lvl3(signer.String(), "round", round, "received the message to be signed from the leader")
		if err != nil {
			dbg.Fatal(signer.String(), "round", round, "received error waiting msg")
		}
		if m.MsgType != MessageSigningType {
			dbg.Fatal(app.RunFlags.Hostname, "round", round, "wanted to receive a msg to sign but..",
				m.MsgType.String())
		}
		msg := m.Msg.(MessageSigning).Msg
		dbg.Lvl3(signer.String(), "round", round, "received msg:", msg[:])
		// Generate signature & send
		s := signer.Signature(msg[:])
		l.Send(ctx, s)
		dbg.Lvl3(signer.String(), "round", round, "sent the signature to leader")
	}
	l.Close()
	dbg.Lvl3(app.RunFlags.Hostname, "Finished")
}

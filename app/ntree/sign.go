package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	net "github.com/dedis/cothority/lib/network_draft/network"
	"sync"
	"sync/atomic"
	"time"
)

func RunServer(conf *app.NTreeConfig) {
	if conf.Root {
		RunRoot(conf)
	} else {
		RunPeer(conf)
	}
}

func RunRoot(conf *app.NTreeConfig) {
	host := net.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)

	peer := NewPeer(host, LeadRole, key.Secret, key.Public)
	dbg.Lvl1(peer.String(), "Up and will make connections...")
	// msg to be sent + signed
	msg := []byte("Hello World")

	// masterRoundChan is used to tell that everyone is ready
	masterRoundChan := make(chan chan chan *net.ListBasicSignature)
	// Open connection for each children
	for _, c := range conf.Tree.Children {
		dbg.Lvl2(peer.String(), "will connect to children ", c.Name)

		conn := peer.Open(c.Name)
		if conn == nil {
			dbg.Fatal(peer.String(), "Could not open connection to child ", c.Name)
		}
		// then start Root protocol
		go func(c net.Conn) {
			dbg.Lvl3(peer.String(), "connected to children", c.PeerName())
			roundSigChan := make(chan chan *net.ListBasicSignature)
			// notify we are ready to begin
			masterRoundChan <- roundSigChan
			// each rounds...
			for lsigChan := range roundSigChan {
				dbg.Lvl4(peer.String(), "starting new round !")
				m := net.MessageSigning{
					Length: len(msg),
					Msg:    msg,
				}
				// send msg to children
				err := c.Send(m)
				if err != nil {
					dbg.Fatal(peer.String(), "could not send message to children ", c.PeerName(), " : ", err)
				}
				dbg.Lvl3(peer.String(), "sent message to children ", c.PeerName())
				// Receive bundled signatures
				app, err := c.Receive()
				if err != nil {
					dbg.Fatal(peer.String(), "could not received bundled signature from ", c.PeerName(), " : ", err)
				}
				if app.MsgType != net.ListBasicSignatureType {
					dbg.Fatal(peer.String(), "received a wrong packet type from ", c.PeerName(), " : ", app.MsgType.String())
				}
				// Then pass them on
				sigs := app.Msg.(net.ListBasicSignature)
				lsigChan <- &sigs
				dbg.Lvl3(peer.String(), "Received list of signatures from child ", c.PeerName())
			}
		}(conn)
	}
	// First collect every "ready-connections"
	rounds := make([]chan chan *net.ListBasicSignature, 0)
	for round := range masterRoundChan {
		rounds = append(rounds, round)
		if len(rounds) == len(conf.Tree.Children) {
			dbg.Lvl3(peer.String(), "collected each children channels")
			break
		}
	}
	close(masterRoundChan)
	// Then for each rounds tell them to start the protocol
	for i := 1; i <= conf.Rounds; i++ {
		dbg.Lvl3(peer.String(), "will start a new round ", i)
		// Start of the round timing
		start := time.Now()
		// the signature channel used for this round
		lsigChan := make(chan *net.ListBasicSignature)
		// notify each connections
		for _, ch := range rounds {
			ch <- lsigChan
		}

		// Wait for listsignatures coming
		dbg.Lvl2(peer.String(), "Waiting on signatures for round ", i, "...")

		var verifyWg sync.WaitGroup
		var faulty uint64 = 0
		var total uint64 = 0
		// how many listsigs have we received
		// == len(children) ? ==> timing !
		var listSigNb int = 0
		// start timing verification
		verify := time.Now()
		for sigs := range lsigChan {
			dbg.Lvl3(peer.String(), "will analyze one ListBasicSignature...")
			listSigNb += 1
			// we have received all bundled signatures so time it
			if listSigNb == len(conf.Tree.Children) {
				log.WithFields(log.Fields{
					"file":  logutils.File(),
					"type":  "ntree_round",
					"round": i,
					"time":  time.Since(start)}).Info("")
				close(lsigChan) // we have finished for this round
			}
			// Here it launches one go routine to verify a bundle
			verifyWg.Add(1)
			go func(s *net.ListBasicSignature) {
				// verify each independant signatures
				for _, sig := range s.Sigs {
					if err := SchnorrVerify(suite, msg, sig); err != nil {
						dbg.Lvl2(peer.String(), "received incorrect signature >< ", err)
						atomic.AddUint64(&faulty, 1)
					}
					atomic.AddUint64(&total, 1)
				}
				verifyWg.Done()
			}(sigs)
		}
		// wait for all verifications
		verifyWg.Wait()
		// finished verifying => time it !
		log.WithFields(log.Fields{
			"file":  logutils.File(),
			"type":  "ntree_verify",
			"round": i,
			"time":  time.Since(verify)}).Info("")
		dbg.Lvl1(peer.String(), "Round ", i, "/", conf.Rounds, " has verified all signatures : ", total-faulty, "/", total, " good signatures")
	}

	// cLosing each channels
	for _, ch := range rounds {
		close(ch)
	}

	log.WithFields(log.Fields{
		"file": logutils.File(),
		"type": "end",
	}).Info("")
	dbg.Lvl2(peer.String(), "leaving ...")
}

func RunPeer(conf *app.NTreeConfig) {

	host := net.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)

	peer := NewPeer(host, ServRole, key.Secret, key.Public)
	dbg.Lvl1(peer.String(), "Up and will make connections...")

	// Chan used to communicate the message from the parent to the children
	// Must do a Fan in to communicate this message to all children
	masterMsgChan := make(chan net.MessageSigning)
	childrenMsgChan := make([]chan net.MessageSigning, len(conf.Tree.Children))
	go func() {
		// for each message
		for msg := range masterMsgChan {
			// broadcast to each channels
			for _, ch := range childrenMsgChan {
				ch <- msg
			}
		}
		// When finished, close all children channs
		for _, ch := range childrenMsgChan {
			close(ch)
		}
	}()
	// chan used to communicate the signature from the children to the parent
	roundSigChan := make(chan chan net.ListBasicSignature)
	// chan used to tell the end of the protocols
	done := make(chan bool)
	// The parent protocol
	proto := func(c net.Conn) {
		dbg.Lvl3(peer.String(), "connected with parent", c.PeerName())
		// for each rounds
		for i := 1; i <= conf.Rounds; i++ {
			// Create the chan for this round
			sigChan := make(chan net.ListBasicSignature)
			// that wil be used for children to pass up their signatures
			roundSigChan <- sigChan
			dbg.Lvl3(peer.String(), "starting round ", i)
			// First, receive the message to be signed
			app, err := c.Receive()
			if err != nil {
				dbg.Fatal(peer.String(), "error receiving message from parent ", c.PeerName())
			}
			if app.MsgType != net.MessageSigningType {
				dbg.Fatal(peer.String(), "received wrong packet type from parent : ", app.MsgType.String())
			}
			msg := app.Msg.(net.MessageSigning)
			// Notify the chan so it will be broadcasted down
			masterMsgChan <- msg
			dbg.Lvl3(peer.String(), "round ", i, " : received message from parent", msg.Msg)
			// issue our signature
			bs := peer.Signature(msg.Msg)
			// wait for children signatures
			sigs := make([]net.BasicSignature, 0)
			sigs = append(sigs, *bs)

			// for each ListBasicSignature
			n := 0
			for lsig := range sigChan {
				// Add each independant signature
				for _, sig := range lsig.Sigs {
					sigs = append(sigs, sig)
				}
				n += 1
				//We got them all ;)
				if n == len(conf.Tree.Children) {
					close(sigChan)
					break
				}
			}

			dbg.Lvl2(peer.String(), "received ", len(sigs), "signatures from children")
			// Then send to parent the signature
			lbs := net.ListBasicSignature{}
			lbs.Length = len(sigs)
			lbs.Sigs = sigs
			err = c.Send(lbs)
			if err != nil {
				dbg.Fatal(peer.String(), "Could not send list of signature to parents ><", err)
			}
			dbg.Lvl2(peer.String(), "round ", i, " : sent the array of sigs to parent")
		}
		close(roundSigChan)
		c.Close()
		done <- true
	}

	dbg.Lvl2(peer.String(), "listen for the parent connection...")
	go peer.Listen(conf.Name, proto)
	// dispatch new round to each children
	childrenSigChan := make([]chan chan net.ListBasicSignature, len(conf.Tree.Children))
	go func() {
		for sigChan := range roundSigChan {
			// if no children, no signature will come
			// so close immediatly so parent connection will continue
			if len(conf.Tree.Children) == 0 {
				close(sigChan)
			} else {
				// otherwise, dispatch to children
				for _, ch := range childrenSigChan {
					ch <- sigChan
				}
			}
		}
		for _, ch := range childrenSigChan {
			close(ch)
		}
	}()

	// Connect to the children
	// Relay the msg
	// Wait for signatures
	dbg.Lvl2(peer.String(), "will contact its siblings..")
	// To stop when every children has done all rounds
	// Connect to every children
	for i, c := range conf.Tree.Children {
		dbg.Lvl3(peer.String(), "is connecting to ", c.Name)
		conn := peer.Open(c.Name)
		if conn == nil {
			dbg.Fatal(peer.String(), "Could not connect to ", c.Name)
		}
		// Children protocol
		go func(child int, c net.Conn) {
			dbg.Lvl3(peer.String(), "is connected to children ", c.PeerName())

			// For each rounds new round
			for sigChan := range childrenSigChan[child] {

				// get & relay the message
				msg := <-childrenMsgChan[child]
				err := c.Send(msg)
				if err != nil {
					dbg.Fatal(peer.String(), "Could not relay message to children ", c.PeerName())
				}
				dbg.Lvl4(peer.String(), "sent to the message to children ", c.PeerName())
				// wait for signature bundle
				app, err := c.Receive()
				if err != nil {
					dbg.Fatal(peer.String(), "Could not receive the bundled children signature from", c.PeerName())
				}
				if app.MsgType != net.ListBasicSignatureType {
					dbg.Fatal(peer.String(), "received an different package from ", c.PeerName(), " : ", app.MsgType.String())
				}
				dbg.Lvl4(peer.String(), "received signature bundle from children ", c.PeerName())
				lbs := app.Msg.(net.ListBasicSignature)
				// send to parent
				sigChan <- lbs
			}
			dbg.Lvl3(peer.String(), "finished with children ", c.PeerName())
			c.Close()
		}(i-1, conn)
	}
	// Wait for the whole thing to be done (parent connection == master)
	<-done
	dbg.Lvl2(peer.String(), "leaving...")

}

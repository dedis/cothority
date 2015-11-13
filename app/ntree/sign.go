package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/monitor"
	net "github.com/dedis/cothority/lib/network"
	"sync"
	"sync/atomic"
)

func RunServer(conf *app.NTreeConfig) {
	if conf.Root {
		RunRoot(conf)
	} else {
		RunPeer(conf)
		//RunServer2(conf)
	}

}

func RunRoot(conf *app.NTreeConfig) {
	host := net.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)

	peer := NewPeer(host, LeadRole, key.Secret, key.Public)
	dbg.Lvl2(peer.String(), "Up and will make connections...")

	// monitor
	if app.RunFlags.Monitor == "" {
		monitor.Disable()
	} else {
		if err := monitor.ConnectSink(app.RunFlags.Monitor); err != nil {
			dbg.Fatal(peer.String(), "could not connect to the monitor : ", err)
		}
	}

	// msg to be sent + signed
	msg := []byte("Hello World")

	// make setup measurement
	setup := monitor.NewMeasure("setup")

	// masterRoundChan is used to tell that everyone is ready
	masterRoundChan := make(chan chan chan *net.ListBasicSignature)
	// Open connection for each children
	for _, c := range conf.Tree.Children {
		dbg.Lvl2(peer.String(), "will connect to children ", c.Name)

		connPeer := peer.Open(c.Name)
		if connPeer == nil {
			dbg.Fatal(peer.String(), "Could not open connection to child ", c.Name)
		}
		// then start Root protocol
		go func(conn net.Conn) {
			dbg.Lvl3(peer.String(), "connected to children", conn.PeerName())
			roundSigChan := make(chan chan *net.ListBasicSignature)
			// notify we are ready to begin
			masterRoundChan <- roundSigChan
			// each rounds...
			for lsigChan := range roundSigChan {
				dbg.Lvl4(peer.String(), "starting new round with", conn.PeerName())
				m := net.MessageSigning{
					Length: len(msg),
					Msg:    msg,
				}
				// send msg to children
				err := conn.Send(m)
				if err != nil {
					dbg.Fatal(peer.String(), "could not send message to children ", conn.PeerName(), " : ", err)
				}
				dbg.Lvl3(peer.String(), "sent message to children ", conn.PeerName())
				// Receive bundled signatures
				sig, err := conn.Receive()
				if err != nil {
					dbg.Fatal(peer.String(), "could not received bundled signature from ", conn.PeerName(), " : ", err)
				}
				if sig.MsgType != net.ListBasicSignatureType {
					dbg.Fatal(peer.String(), "received a wrong packet type from ", conn.PeerName(), " : ", sig.MsgType.String())
				}
				// Then pass them on
				sigs := sig.Msg.(net.ListBasicSignature)
				lsigChan <- &sigs
				dbg.Lvl3(peer.String(), "Received list of signatures from child ", conn.PeerName())
			}
		}(connPeer)
	}
	// First collect every "ready-connections"
	children := make([]chan chan *net.ListBasicSignature, 0)
	for round := range masterRoundChan {
		children = append(children, round)
		if len(children) == len(conf.Tree.Children) {
			dbg.Lvl3(peer.String(), "collected each children channels")
			break
		}
	}
	close(masterRoundChan)
	setup.Measure()

	// Then for each rounds tell them to start the protocol
	round := monitor.NewMeasure("round")
	for i := 1; i <= conf.Rounds; i++ {
		dbg.Lvl3(peer.String(), "will start a new round ", i)
		calc := monitor.NewMeasure("calc")
		// the signature channel used for this round
		lsigChan := make(chan *net.ListBasicSignature)
		// notify each connections
		for _, ch := range children {
			ch <- lsigChan
		}

		childrenSigs := make([]*net.ListBasicSignature, 0)
		// Wait for listsignatures coming
		dbg.Lvl2(peer.String(), "Waiting on signatures for round ", i, "...")

		for sigs := range lsigChan {
			dbg.Lvl3(peer.String(), "will analyze one ListBasicSignature...")
			childrenSigs = append(childrenSigs, sigs)
			// we have received all bundled signatures so time it
			if len(childrenSigs) == len(conf.Tree.Children) {
				close(lsigChan) // we have finished for this round
			}
		}
		dbg.Lvl2(peer.String(), "Received all signatures ... ")
		calc.Measure()

		var verifyWg sync.WaitGroup
		var faulty uint64 = 0
		var total uint64 = 0
		// start timing verification
		verify := monitor.NewMeasure("verify")
		for _, sigs := range childrenSigs {
			// Here it launches one go routine to verify a bundle
			verifyWg.Add(1)
			go func(s *net.ListBasicSignature) {
				defer verifyWg.Done()
				if conf.SkipChecks {
					return
				}
				// verify each independant signatures
				for _, sig := range s.Sigs {
					if err := SchnorrVerify(suite, msg, sig); err != nil {
						dbg.Lvl2(peer.String(), "received incorrect signature >< ", err)
						atomic.AddUint64(&faulty, 1)
					}
					atomic.AddUint64(&total, 1)
				}
			}(sigs)
		}
		// wait for all verifications
		verifyWg.Wait()
		// finished verifying => time it !
		verify.Measure()
		round.Measure()
		dbg.Lvl2(peer.String(), "Round ", i, "/", conf.Rounds, " has verified all signatures : ", total-faulty, "/", total, " good signatures")
	}

	// cLosing each channels
	for _, ch := range children {
		close(ch)
	}

	monitor.End()
	dbg.Lvl1(peer.String(), "Finished all rounds successfully.")
}

func RunPeer(conf *app.NTreeConfig) {

	host := net.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)

	peer := NewPeer(host, ServRole, key.Secret, key.Public)
	dbg.Lvl2(peer.String(), "Up and will make connections...")

	// Chan used to communicate the message from the parent to the children
	// Must do a Fan out to communicate this message to all children
	masterMsgChan := make(chan net.MessageSigning)
	childrenMsgChan := make([]chan net.MessageSigning, len(conf.Tree.Children))
	go func() {
		// init
		for i := range childrenMsgChan {
			childrenMsgChan[i] = make(chan net.MessageSigning)
		}
		// for each message
		for msg := range masterMsgChan {
			// broadcast to each channels
			for i, ch := range childrenMsgChan {
				dbg.Lvl4(peer.String(), "dispatching msg to children (", i+1, "/", len(conf.Tree.Children), ")...")
				ch <- msg
			}
		}
		// When finished, close all children channs
		for _, ch := range childrenMsgChan {
			close(ch)
		}
	}()

	// chan used to communicate the signature from the children to the parent
	// It is also used to specify the start of a new round (coming from the parent
	// connection)
	masterRoundChan := make(chan chan net.ListBasicSignature)
	// dispatch new round to each children
	childRoundChan := make([]chan chan net.ListBasicSignature, len(conf.Tree.Children))
	dbg.Lvl3(peer.String(), "created children Signal Channels (length = ", len(childRoundChan), ")")
	go func() {
		// init
		for i := range childRoundChan {
			childRoundChan[i] = make(chan chan net.ListBasicSignature)
		}
		// For each new round started by the parent's connection
		for sigChan := range masterRoundChan {
			// if no children, no signature will come
			// so close immediatly so parent connection will continue
			if len(conf.Tree.Children) == 0 {
				dbg.Lvl3(peer.String(), "Has no children so closing childRoundChan")
				close(sigChan)
			} else {
				// otherwise, dispatch to children
				for i, _ := range childRoundChan {
					dbg.Lvl4(peer.String(), "Dispatching signature channel to children (", i+1, "/", len(conf.Tree.Children), ")...")
					childRoundChan[i] <- sigChan
				}
			}
		}
		dbg.Lvl3(peer.String(), "closing the children sig channels...")
		for _, ch := range childRoundChan {
			close(ch)
		}
	}()

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
			masterRoundChan <- sigChan
			dbg.Lvl3(peer.String(), "starting round ", i)
			// First, receive the message to be signed
			sig, err := c.Receive()
			if err != nil {
				dbg.Fatal(peer.String(), "error receiving message from parent ", c.PeerName())
			}
			if sig.MsgType != net.MessageSigningType {
				dbg.Fatal(peer.String(), "received wrong packet type from parent : ", sig.MsgType.String())
			}
			msg := sig.Msg.(net.MessageSigning)
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
			dbg.Lvl3(peer.String(), "round ", i, " : waiting on signatures from children ...")
			for lsig := range sigChan {
				dbg.Lvl3(peer.String(), "round", i, " : receievd a ListSignature !")
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
		close(masterRoundChan)
		c.Close()
		done <- true
	}

	dbg.Lvl2(peer.String(), "listen for the parent connection...")
	go peer.Listen(conf.Name, proto)

	// Connect to the children
	// Relay the msg
	// Wait for signatures
	dbg.Lvl2(peer.String(), "will contact its siblings..")
	// To stop when every children has done all rounds
	// Connect to every children
	for i, c := range conf.Tree.Children {
		dbg.Lvl3(peer.String(), "is connecting to ", c.Name, "(", i, ")")
		connPeer := peer.Open(c.Name)
		if connPeer == nil {
			dbg.Fatal(peer.String(), "Could not connect to ", c.Name)
		}
		// Children protocol
		go func(child int, conn net.Conn) {
			dbg.Lvl3(peer.String(), "is connected to children ", conn.PeerName(), "(", child, ")")

			// For each rounds new round
			for sigChan := range childRoundChan[child] {
				dbg.Lvl3(peer.String(), "starting new round with children ", conn.PeerName(), "(", child, ")")
				// get & relay the message
				msg := <-childrenMsgChan[child]
				dbg.Lvl3(peer.String(), "will relay message to child ", conn.PeerName(), "(", child, ")")
				err := conn.Send(msg)
				if err != nil {
					dbg.Fatal(peer.String(), "Could not relay message to children ", conn.PeerName())
				}
				dbg.Lvl4(peer.String(), "sent to the message to children ", conn.PeerName())
				// wait for signature bundle
				sig, err := conn.Receive()
				if err != nil {
					dbg.Fatal(peer.String(), "Could not receive the bundled children signature from", conn.PeerName())
				}
				if sig.MsgType != net.ListBasicSignatureType {
					dbg.Fatal(peer.String(), "received an different package from ", conn.PeerName(), " : ", sig.MsgType.String())
				}
				dbg.Lvl4(peer.String(), "received signature bundle from children ", conn.PeerName())
				lbs := sig.Msg.(net.ListBasicSignature)
				// send to parent
				sigChan <- lbs
			}
			dbg.Lvl3(peer.String(), "finished with children ", conn.PeerName())
			conn.Close()
		}(i, connPeer)
	}
	// Wait for the whole thing to be done (parent connection == master)
	<-done
	dbg.Lvl2(peer.String(), "leaving...")

}

func RunServer2(conf *app.NTreeConfig) {

	host := net.NewTcpHost(app.RunFlags.Hostname)
	key := cliutils.KeyPair(suite)

	peer := NewPeer(host, ServRole, key.Secret, key.Public)
	dbg.Lvl2(peer.String(), "Up and will make connections...")

	nChildren := len(conf.Tree.Children)
	dbg.Lvl3(peer.String(), "starting with ", nChildren, "children")
	// Channel that will be used to transmit messages down to the children conn
	masterMsgChan := make(chan net.MessageSigning)
	// mster will broadcast to these children channels
	childrenMsgChan := make([]chan net.MessageSigning, nChildren)

	// make the fan out routine
	dbg.Lvl3(peer.String(), "launching the fan-out channels")
	go func() {
		// init
		for i := range childrenMsgChan {
			childrenMsgChan[i] = make(chan net.MessageSigning)
		}
		for sig := range masterMsgChan {
			for _, ch := range childrenMsgChan {
				ch <- sig
			}
		}
		for _, ch := range childrenMsgChan {
			close(ch)
		}
	}()

	// Channel that will be used to transmit signature from children
	// to parent connection
	masterSigChan := make(chan net.ListBasicSignature)
	// if we are a leaf
	if nChildren == 0 {
		// no need to wait for childrens signature!
		dbg.Lvl3(peer.String(), "has no children: closing masterSigChan")
		close(masterSigChan)
	}

	// channel to signal the end of protocol
	done := make(chan bool)
	// protocol for the parent <-> peer connection
	parent := func(c net.Conn) {
		// for each round
		dbg.Lvl2(peer.String(), "connected to parent : ", c.PeerName())
		for i := 0; i < conf.Rounds; i++ {
			dbg.Lvl3(peer.String(), "starting new round ", i, " with parent ", c.PeerName())
			// Receive message
			msg := peer.ReceiveMessage(c)
			// braodcast down
			masterMsgChan <- msg

			dbg.Lvl3(peer.String(), "received msg from parent & broadcasted msg to children")

			// wait for list basic signatures from children
			sigs := make([]net.BasicSignature, 0)
			// append its own signature
			own := peer.Signature(msg.Msg)
			sigs = append(sigs, *own)
			n := 0
			for lbs := range masterSigChan {
				dbg.Lvl4(peer.String(), "appending a new LBS from child")
				// append all individual signatures
				for _, sig := range lbs.Sigs {
					sigs = append(sigs, sig)
				}
				n += 1
				if n == nChildren {
					// all sigs received
					break
				}
			}
			dbg.Lvl4(peer.String(), "received all LBS from children")

			// send back to parent
			lbs := net.ListBasicSignature{
				Length: len(sigs),
				Sigs:   sigs,
			}
			if err := c.Send(lbs); err != nil {
				dbg.Fatal(peer.String(), "could not sent the aggregate signatures to parent ", c.PeerName())
			}
		}
		// finished
		c.Close()
		done <- true
	}
	// go listen
	go peer.Listen(peer.Name(), parent)

	// the protocol for the peer <-> children connection
	children := func(c net.Conn, msgChan chan net.MessageSigning) {
		dbg.Lvl2(peer.String(), "connected with children ", c.PeerName())
		for i := 0; i < conf.Rounds; i++ {
			dbg.Lvl3(peer.String(), "(", i, ") waiting upstream message for ", c.PeerName())
			// wait for the message
			msg := <-msgChan
			// send it
			if err := c.Send(msg); err != nil {
				dbg.Fatal(peer.String(), "(", i, ") could not send msg down to children ", c.PeerName(), " : ", err)
			}
			dbg.Lvl3(peer.String(), "(", i, ") sent upstream msg to ", c.PeerName())

			// wait for the response sigs
			lbs := peer.ReceiveListBasicSignature(c)
			dbg.Lvl3(peer.String(), "(", i, ") received ListBasicSignature from children : ", c.PeerName())
			// dispatch to the parent conn
			masterSigChan <- lbs
		}
		dbg.Lvl3(peer.String(), " conn with children will close.")
		c.Close()
	}

	// launch the children connections
	for i := 0; i < nChildren; i++ {
		c := peer.Open(conf.Tree.Children[i].Name)
		if c == nil {
			dbg.Fatal(peer.String(), "could not open connection to ", conf.Tree.Children[i].Name)
		}
		go children(c, childrenMsgChan[i])
	}

	// wait for the end
	dbg.Lvl2(peer.String(), "waiting the end of the rounds ...")
	<-done
	close(masterMsgChan)
	dbg.Lvl2(peer.String(), "is finished !")
}

package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/poly"
)

func RunServer(conf *app.ConfigShamir) {
	flags := app.RunFlags
	s := edwards.NewAES128SHA256Ed25519(false)
	n := len(conf.Hosts)

	info := poly.Threshold{
		N: n,
		R: n,
		T: n,
	}
	indexPeer := -1
	for i, h := range conf.Hosts {
		if h == flags.Hostname {
			indexPeer = i
			break
		}
	}
	if indexPeer == -1 {
		log.Fatal("Peer ", flags.Hostname, "(", flags.PhysAddr, ") did not find any match for its name.Abort")
	}

	dbg.Lvl3("Creating new peer ", flags.Hostname, "(", flags.PhysAddr, ") ...")
	// indexPeer == 0 <==> peer is root
	p := NewPeer(indexPeer, flags.Hostname, s, info, indexPeer == 0)

	// make it listen
	setup := monitor.NewMeasure("setup")
	dbg.Lvl3("Peer", flags.Hostname, "is now listening for incoming connections")
	go p.Listen()

	// then connect it to its successor in the list
	for _, h := range conf.Hosts[indexPeer+1:] {
		dbg.Lvl3("Peer ", flags.Hostname, " will connect to ", h)
		// will connect and SYN with the remote peer
		p.ConnectTo(h)
	}
	// Wait until this peer is connected / SYN'd with each other peer
	p.WaitSYNs()

	// Setup the schnorr system amongst peers
	p.SetupDistributedSchnorr()
	p.SendACKs()
	p.WaitACKs()
	dbg.Lvl3(p.String(), "completed Schnorr setup")

	// send setup time if we're root
	if p.IsRoot() {
		setup.Measure()
	}

	roundm := monitor.NewMeasure("round")
	for round := 1; round <= conf.Rounds; round++ {
		calc := monitor.NewMeasure("calc")
		// Then issue a signature !
		//sys, usr := app.GetRTime()
		msg := "hello world"

		// Only root calculates if it's OK and sends a log-message
		if p.IsRoot() {
			dbg.Lvl1("Starting round", round)
			sig := p.SchnorrSigRoot([]byte(msg))
			calc.Measure()
			verify := monitor.NewMeasure("verify")
			err := p.VerifySchnorrSig(sig, []byte(msg))
			if err != nil {
				dbg.Fatal(p.String(), "could not verify schnorr signature :/ ", err)
			}
			verify.Measure()
			roundm.Measure()
			dbg.Lvl3(p.String(), "verified the schnorr sig !")
		} else {
			// Compute the partial sig and send it to the root
			p.SchnorrSigPeer([]byte(msg))
		}
	}

	p.WaitFins()
	dbg.Lvl3(p.String(), "is leaving ...")

	if p.IsRoot() {
		monitor.End()
	}
}

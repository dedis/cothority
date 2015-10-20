package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/cothority/lib/monitor"
)

func RunServer(conf *app.ConfigShamir) {
	flags := app.RunFlags
	s := app.GetSuite(conf.Suite)
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

	dbg.Lvl2("Creating new peer ", flags.Hostname, "(", flags.PhysAddr, ") ...")
	// indexPeer == 0 <==> peer is root
	p := NewPeer(indexPeer, flags.Hostname, s, info, indexPeer == 0)

	// make it listen
	measure := monitor.NewMeasure()
	dbg.Lvl2("Peer", flags.Hostname, "is now listening for incoming connections")
	go p.Listen()

	// then connect it to its successor in the list
	for _, h := range conf.Hosts[indexPeer+1:] {
		dbg.Lvl2("Peer ", flags.Hostname, " will connect to ", h)
		// will connect and SYN with the remote peer
		p.ConnectTo(h)
	}
	// Wait until this peer is connected / SYN'd with each other peer
	p.WaitSYNs()

	if p.IsRoot() {
		measure.MeasureWall("basic_connect")
	}

	// Setup the schnorr system amongst peers
	p.SetupDistributedSchnorr()
	p.SendACKs()
	p.WaitACKs()
	dbg.Lvl2(p.String(), "completed Schnorr setup")

	// send setup time if we're root
	if p.IsRoot() {
		measure.MeasureWall("basic_setup")
	}

	for round := 0; round < conf.Rounds; round++ {
		if p.IsRoot() {
			dbg.Lvl2("Starting round", round)
		}

		measure.Update()
		measure_wall := monitor.NewMeasure()
		// Then issue a signature !
		//sys, usr := app.GetRTime()
		msg := "hello world"

		// Only root calculates if it's OK and sends a log-message
		if p.IsRoot() {
			sig := p.SchnorrSigRoot([]byte(msg))
			measure.MeasureCPU("basic_calc")

			err := p.VerifySchnorrSig(sig, []byte(msg))
			measure.MeasureCPU("basic_verify")
			if err != nil {
				dbg.Fatal(p.String(), "could not verify schnorr signature :/ ", err)
			}

			dbg.Lvl2(p.String(), "verified the schnorr sig !")
			measure_wall.MeasureWall("basic_round")
		} else {
			// Compute the partial sig and send it to the root
			p.SchnorrSigPeer([]byte(msg))
		}
	}

	p.WaitFins()
	dbg.Lvl2(p.String(), "is leaving ...")

	if p.IsRoot() {
		monitor.LogEnd()
	}
}

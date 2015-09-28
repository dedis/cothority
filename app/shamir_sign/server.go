package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/lib/config"
	"github.com/dedis/crypto/poly"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"time"
	"github.com/dedis/cothority/lib/app"
)

func RunServer(hosts *config.HostsConfig, ac *app.AppConfig) {
	app := ac.Flags
	depl := ac.Conf
	s := config.GetSuite(depl.Suite)
	poly.SUITE = s
	poly.SECURITY = poly.MODERATE
	n := len(hosts.Hosts)

	info := poly.PolyInfo{
		N: n,
		R: n,
		T: n,
	}
	indexPeer := -1
	for i, h := range hosts.Hosts {
		if h == app.Hostname {
			indexPeer = i
			break
		}
	}
	if indexPeer == -1 {
		log.Fatal("Peer ", app.Hostname, "(", app.PhysAddr, ") did not find any match for its name.Abort")
	}

	start := time.Now()
	dbg.Lvl2("Creating new peer ", app.Hostname, "(", app.PhysAddr, ") ...")
	// indexPeer == 0 <==> peer is root
	p := NewPeer(indexPeer, app.Hostname, info, indexPeer == 0)

	// make it listen
	dbg.Lvl2("Peer", app.Hostname, "is now listening for incoming connections")
	go p.Listen()

	// then connect it to its successor in the list
	for _, h := range hosts.Hosts[indexPeer + 1:] {
		dbg.Lvl2("Peer ", app.Hostname, " will connect to ", h)
		// will connect and SYN with the remote peer
		p.ConnectTo(h)
	}
	// Wait until this peer is connected / SYN'd with each other peer
	p.WaitSYNs()

	if p.IsRoot() {
		delta := time.Since(start)
		dbg.Lvl2(p.String(), "Connections accomplished in", delta)
		log.WithFields(log.Fields{
			"file":  logutils.File(),
			"type":  "schnorr_connect",
			"round": 0,
			"time":  delta,
		}).Info("")
	}

	// start to record
	start = time.Now()

	// Setup the schnorr system amongst peers
	p.SetupDistributedSchnorr()
	p.SendACKs()
	p.WaitACKs()
	dbg.Lvl2(p.String(), "completed Schnorr setup")

	// send setup time if we're root
	if p.IsRoot() {
		delta := time.Since(start)
		dbg.Lvl2(p.String(), "setup accomplished in ", delta)
		log.WithFields(log.Fields{
			"file":  logutils.File(),
			"type":  "schnorr_setup",
			"round": 0,
			"time":  delta,
		}).Info("")
	}

	for round := 0; round < depl.Rounds; round++ {
		if p.IsRoot() {
			dbg.Lvl2("Starting round", round)
		}

		// Then issue a signature !
		start = time.Now()
		msg := "hello world"

		// Only root calculates if it's OK and sends a log-message
		if p.IsRoot() {
			sig := p.SchnorrSigRoot([]byte(msg))
			err := p.VerifySchnorrSig(sig, []byte(msg))
			if err != nil {
				dbg.Fatal(p.String(), "could not verify schnorr signature :/ ", err)
			}

			dbg.Lvl2(p.String(), "verified the schnorr sig !")
			// record time
			delta := time.Since(start)
			dbg.Lvl2(p.String(), "signature done in ", delta)
			log.WithFields(log.Fields{
				"file":  logutils.File(),
				"type":  "schnorr_round",
				"round": round,
				"time":  delta,
			}).Info("")
		} else {
			// Compute the partial sig and send it to the root
			p.SchnorrSigPeer([]byte(msg))
		}
	}

	p.WaitFins()
	dbg.Lvl2(p.String(), "is leaving ...")

	if p.IsRoot() {
		log.WithFields(log.Fields{
			"file": logutils.File(),
			"type":    "schnorr_end",
		}).Info("")
	}
}

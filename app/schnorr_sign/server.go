package schnorr_sign

import log "github.com/Sirupsen/logrus"
import "github.com/dedis/cothority/deploy"
import "github.com/dedis/cothority/lib/config"
import "github.com/dedis/crypto/poly"
import dbg "github.com/dedis/cothority/lib/debug_lvl"

func RunServer(hosts *config.HostsConfig, app *config.AppConfig, depl *deploy.Config) {
	s := config.GetSuite(depl.Suite)

	n := len(hosts.Hosts)

	info := poly.PolyInfo{
		N:     n,
		R:     n,
		T:     n,
		Suite: s,
	}
	indexPeer := -1
	for i, h := range hosts.Hosts {
		if h == app.PhysAddr {
			indexPeer = i
		}
	}
	if indexPeer == -1 {
		log.Fatal("Peer ", app.Hostname, "(", app.PhysAddr, ") did not find any match for its name.Abort")
	}

	dbg.Lvl1("Creating new peer ", app.Hostname, "(", app.PhysAddr, ") ...")
	p := NewPeer(indexPeer, app.Hostname, info)

	// make it listen
	dbg.Lvl2("Peer ", app.Hostname, "is now listening for incoming connections")
	go p.Listen()

	// then connect it to its successor in the list
	for _, h := range hosts.Hosts[indexPeer:] {
		p.ConnectTo(h)
	}
}

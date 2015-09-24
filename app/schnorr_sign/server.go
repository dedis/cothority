package schnorr_sign

import log "github.com/Sirupsen/logrus"
import "github.com/dedis/cothority/deploy"
import "github.com/dedis/cothority/lib/config"
import "github.com/dedis/crypto/poly"
import dbg "github.com/dedis/cothority/lib/debug_lvl"

func RunServer(hosts *config.HostsConfig, app *config.AppConfig, depl *deploy.Config) {
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

	dbg.Lvl2("Creating new peer ", app.Hostname, "(", app.PhysAddr, ") ...")
	// indexPeer == 0 <==> peer is root
	p := NewPeer(indexPeer, app.Hostname, info, indexPeer == 0)

	// make it listen
	dbg.Lvl2("Peer ", app.Hostname, "is now listening for incoming connections")
	go p.Listen()

	// then connect it to its successor in the list
	for _, h := range hosts.Hosts[indexPeer+1:] {
		dbg.Lvl2("Peer ", app.Hostname, " will connect to ", h)
		// will connect and SYN with the remote peer
		p.ConnectTo(h)
	}
	// Wait until this peer is connected / SYN'd with each other peer
	p.WaitSYNs()
	// Wait until this peer knows that each other peer is also SYN'd
	p.SendACKs()
	p.WaitACKs()
	// Setup the schnorr system amongst peers
	p.SetupDistributedSchnorr()
	p.SendACKs()
	p.WaitACKs()
	// Then issue a signature !
	msg := "hello world"
	sig := p.SchnorrSig([]byte(msg))
	//err := p.VerifySchnorrSig(sig)
	arr := p.BroadcastSignature(sig)
	for i, _ := range arr {
		err := p.VerifySchnorrSig(arr[i], []byte(msg))
		if err != nil {
			dbg.Fatal(p.String(), "could not verify issued schnorr signature : ", err)
		}
	}
	dbg.Lvl1(p.String(), "verified ALL schnorr sig !")

	dbg.Lvl1("Peer ", app.Hostname, "is leaving ...")
}

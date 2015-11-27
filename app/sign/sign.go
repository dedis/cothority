package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/monitor"
)

func main() {
	conf := &app.ConfigColl{}
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		dbg.Fatal("Hostname empty : Abort")
	}

	// Do some common setup
	if app.RunFlags.Mode == "client" {
		app.RunFlags.Hostname = app.RunFlags.Name
	}
	hostname := app.RunFlags.Hostname
	if hostname == conf.Hosts[0] {
		dbg.Lvlf3("Tree is %+v", conf.Tree)
	}
	dbg.Lvl3(hostname, "Starting to run")

	peer := conode.NewPeer(hostname, conf.ConfigConode)
	app.RunFlags.StartedUp(len(conf.Hosts))
	peer.LoopRounds(RoundMeasureType, conf.Rounds)
	dbg.Lvlf3("Done - flags are %+v", app.RunFlags)
	if app.RunFlags.AmRoot {
		monitor.End()
	}
}

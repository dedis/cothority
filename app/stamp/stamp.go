package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/conode"
)

func main() {
	conf := &app.ConfigColl{}
	app.ReadConfig(conf)

	switch app.RunFlags.Mode {
	case "server":
		RunServer(&app.RunFlags, conf)
	case "client":
		RunClient(&app.RunFlags, conf)
	}
}

func RunServer(flags *app.Flags, conf *app.ConfigColl) {
	hostname := flags.Hostname
	if hostname == conf.Hosts[0] {
		dbg.Lvlf3("Tree is %+v", conf.Tree)
	}
	dbg.Lvl3(hostname, "Starting to run")

	peer := conode.NewPeer(hostname, conf.ConfigConode)
	flags.StartedUp(len(conf.Hosts))
	peer.LoopRounds(RoundMeasureType, conf.Rounds)
	dbg.Lvlf3("Done - flags are %+v", flags)
	monitor.End()
}

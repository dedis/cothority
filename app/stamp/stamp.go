package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
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

	app.RunFlags.StartedUp(len(conf.Hosts))
	peer := conode.NewPeer(hostname, conf.ConfigConode)

	if app.RunFlags.AmRoot {
		err := peer.WaitRoundSetup(len(conf.Hosts), 5, 2)
		if err != nil {
			dbg.Fatal(err)
		}
	}

	RegisterRoundMeasure(peer.Node.LastRound())
	peer.LoopRounds(RoundMeasureType, conf.Rounds)
	dbg.Lvlf3("Done - flags are %+v", app.RunFlags)
	monitor.End()
}

package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sign"
	"time"
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
	peer.SetupConnections()

	if app.RunFlags.AmRoot {
		for {
			setupRound := sign.NewRoundSetup(peer.Node)
			if err := peer.StartAnnouncementWithWait(setupRound, 5*time.Second); err != nil {
				continue
			}
			counted := <-setupRound.Counted
			dbg.Lvl1("Number of peers counted:", counted)
			if counted == len(conf.Hosts) {
				dbg.Lvl1("All hosts replied")
				break
			}
			time.Sleep(time.Second)
		}
	}

	RegisterRoundMeasure(peer.Node.LastRound())
	peer.LoopRounds(RoundMeasureType, conf.Rounds)
	dbg.Lvlf3("Done - flags are %+v", app.RunFlags)
	monitor.End()
}

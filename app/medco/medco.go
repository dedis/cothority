package main

import (
	"fmt"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sign"
	"time"
)

func init() {

}

func main() {
	//RoundMedcoType := RoundMedcoCompareType
	RoundMedcoType := RoundMedcoBucketType

	conf := &app.ConfigColl{}
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		dbg.Fatal("Hostname empty: Abort")
	}

	// Do some common setup
	if app.RunFlags.Mode == "client" {
		app.RunFlags.Hostname = app.RunFlags.Name
	}
	hostname := app.RunFlags.Hostname
	if hostname == conf.Hosts[0] {
		dbg.Lvlf3("Tree is %+v", conf.Tree)
	}

	roundMedcoBase := NewRoundMedco()
	sign.RegisterRoundFactory(RoundMedcoBucketType,
		func(node *sign.Node) sign.Round {
			dbg.Lvl3("Making new RoundMedcoBucket", node.Name())
			round := &RoundMedcoBucket{RoundMedco: roundMedcoBase}
			round.RoundStruct = sign.NewRoundStruct(node, RoundMedcoBucketType)
			return round
		})

	sign.RegisterRoundFactory(RoundMedcoCompareType,
		func(node *sign.Node) sign.Round {
			dbg.Lvl3("Making new RoundMedcoCompare", node.Name())
			round := &RoundMedcoCompare{RoundMedco: roundMedcoBase}
			round.RoundStruct = sign.NewRoundStruct(node, RoundMedcoCompareType)
			return round
		})

	dbg.Lvl3(hostname, "Starting to run")

	start_total := time.Now()

	app.RunFlags.StartedUp(len(conf.Hosts))
	peer := conode.NewPeer(hostname, conf.ConfigConode)

	if app.RunFlags.AmRoot {
		for {
			time.Sleep(time.Second)
			setupRound := sign.NewRoundSetup(peer.Node)
			dbg.Lvl1("Starting StartAnnouncementWithWait")
			if err := peer.StartAnnouncementWithWait(setupRound, 2*60*time.Second); err != nil {
				dbg.Lvl1("Error while doing StartAnnouncement", err)
			}
			dbg.Lvl1("End StartAnnouncementWithWait")

			counted := <-setupRound.Counted
			dbg.Lvl1("Number of peers counted:", counted)
			if counted == len(conf.Hosts) {
				dbg.Lvl1("All hosts replied")
				break
			}
		}
	}

	if app.RunFlags.AmRoot {
		round, err := sign.NewRoundFromType(RoundMedcoType, peer.Node)
		if err != nil {
			dbg.Fatal("Couldn't create", RoundMedcoType, "round:", err)
		}
		//peer.StartAnnouncement(round)
		peer.StartAnnouncementWithWait(round, time.Minute*60)
		peer.SendCloseAll()
	} else {
		peer.LoopRounds(RoundMedcoType, conf.Rounds)
	}

	dbg.Lvlf3("Done - flags are %+v", app.RunFlags)
	monitor.End()
	elapsed_total := time.Since(start_total)
	fmt.Println("elapsed_total\n", elapsed_total)
}

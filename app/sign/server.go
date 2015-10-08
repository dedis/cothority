package main
import (
	"time"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/cothority/lib/app"
)

func RunServer(conf *app.ConfigColl, hc *graphs.HostConfig) {
	// run this specific host
	err := hc.Run(false, sign.MerkleTree, app.RunFlags.Hostname)
	if err != nil {
		log.Fatal(err)
	}

	dbg.Lvl3(app.RunFlags.Hostname, "started up in server-mode")

	// Let's start the client if we're the root-node
	if hc.SNodes[0].IsRoot(0) {
		dbg.Lvl1(app.RunFlags.Hostname, "started client")
		RunClient(conf, hc)
	} else{
		// Endless-loop till we stop by tearing down the connections
		for {
			time.Sleep(time.Minute)
		}
	}
}

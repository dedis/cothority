package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/proto/sign"
	"time"
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
		dbg.Lvl2(app.RunFlags.Hostname, "started client")
		if app.RunFlags.Monitor == "" {
			monitor.Disable()
		} else {
			if err := monitor.ConnectSink(app.RunFlags.Monitor); err != nil {
				dbg.Fatal("Signing root error connecting to monitor :", err)
			}
		}
		RunClient(conf, hc)
	} else {
		// Endless-loop till we stop by tearing down the connections
		for !hc.SNodes[0].Isclosed {
			time.Sleep(time.Second)
		}
	}
}

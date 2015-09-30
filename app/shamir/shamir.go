package main

import (
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/app"
)

// Dispatch-function for running either client or server (mode-parameter)
func main() {
	conf := &app.ConfigShamir{}
	app.ReadConfig(conf)

	// we must know who we are
	if app.Flags.Hostname == "" {
		log.Fatal("Hostname empty : Abort")
	}

	dbg.Lvl2(app.Flags.Hostname, "Starting to run as ", app.Flags.Mode)
	switch app.Flags.Mode {
	case "client":
		RunClient(conf)
	case "server":
		RunServer(conf)
	}
}

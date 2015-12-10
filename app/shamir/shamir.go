package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
)

// Dispatch-function for running either client or server (mode-parameter)
func main() {
	conf := &app.ConfigShamir{}
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		dbg.Fatal("Hostname empty: Abort")
	}

	dbg.Lvl2(app.RunFlags.Hostname, "Starting to run as", app.RunFlags.Mode)
	switch app.RunFlags.Mode {
	case "client":
		RunClient(conf)
	case "server":
		RunServer(conf)
	}
}

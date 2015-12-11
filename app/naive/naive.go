package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
)

func main() {

	conf := new(app.NaiveConfig)
	app.ReadConfig(conf)

	if app.RunFlags.Hostname == "" {
		dbg.Fatal("Hostname empty: Abort")
	}

	RunServer(conf)
	//monitor.End()
}

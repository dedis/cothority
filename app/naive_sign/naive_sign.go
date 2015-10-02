package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

func main() {

	conf := new(app.NaiveSign)
	app.ReadConfig(conf)

	if app.RunFlags.hostname == "" {
		log.Fatal("Hostname empty : Abort")
	}

	dbg.Lvl2(app.RunFlags.Hostname, "starting to run as ", app.RunFlags.Mode)
	switch app.RunFlags.Mode {
	case "client":
		break
	case "server":
		RunServer(conf)
	}

}

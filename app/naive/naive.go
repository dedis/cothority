package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/monitor"
)

func main() {

	conf := new(app.NaiveConfig)
	app.ReadConfig(conf)

	if app.RunFlags.Hostname == "" {
		log.Fatal("Hostname empty : Abort")
	}

	RunServer(conf)
	monitor.End()
}

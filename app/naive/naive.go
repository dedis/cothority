package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
)

func main() {

	conf := new(app.NaiveConfig)
	app.ReadConfig(conf)

	// TODO option configuration
	monitor.ConnectSink("users.deterlab.net:4000")

	if app.RunFlags.Hostname == "" {
		log.Fatal("Hostname empty : Abort")
	}

	RunServer(conf)

}

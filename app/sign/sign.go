package main

import (
	"github.com/dedis/cothority/lib/app"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/proto/sign"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// Dispatch-function for running either client or server (mode-parameter)
func main() {
	conf := &app.ConfigColl{}
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		log.Fatal("Hostname empty : Abort")
	}

	// Do some common setup
	if app.RunFlags.Mode == "client" {
		app.RunFlags.Hostname = app.RunFlags.Name
	}
	hostname := app.RunFlags.Hostname
	if hostname == conf.Hosts[0] {
		dbg.Lvlf3("Tree is %+v", conf.Tree.String(0))
	}
	dbg.Lvl3(app.RunFlags.Hostname, "Starting to run")
	if conf.Debug > 1 {
		sign.DEBUG = true
	}

	if hostname == "" {
		log.Fatal("no hostname given", hostname)
	}

	// load the configuration
	dbg.Lvl3("loading configuration for", hostname)
	var hc *graphs.HostConfig
	var err error
	s := app.GetSuite(conf.Suite)
	opts := graphs.ConfigOptions{ConnType: "tcp", Host: hostname, Suite: s}
	if conf.Failures > 0 || conf.FFail > 0 {
		opts.Faulty = true
	}
	hc, err = graphs.LoadConfig(conf.Hosts, conf.Tree, opts)
	if err != nil {
		log.Fatal(err)
	}

	// Wait for everybody to be ready before going on
	ioutil.WriteFile("coll_stamp_up/up"+hostname, []byte("started"), 0666)
	for {
		_, err := os.Stat("coll_stamp_up")
		if err == nil {
			files, _ := ioutil.ReadDir("coll_stamp_up")
			dbg.Lvl4(hostname, "waiting for others to finish", len(files))
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	dbg.Lvl2(hostname, "thinks everybody's here")

	// set FailureRates
	if conf.Failures > 0 {
		for i := range hc.SNodes {
			hc.SNodes[i].FailureRate = conf.Failures
		}
	}

	// set root failures
	if conf.RFail > 0 {
		for i := range hc.SNodes {
			hc.SNodes[i].FailAsRootEvery = conf.RFail

		}
	}
	// set follower failures
	// a follower fails on %ffail round with failureRate probability
	for i := range hc.SNodes {
		hc.SNodes[i].FailAsFollowerEvery = conf.FFail
	}

	switch app.RunFlags.Mode {
	case "client":
		log.Panic("No client mode")
	case "server":
		RunServer(conf, hc)
	}
	dbg.Lvl2("Collective Signing", hostname, "has terminated in mode", app.RunFlags.Mode)
}

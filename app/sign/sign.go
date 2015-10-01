package main

import (
	"fmt"
	"github.com/dedis/cothority/lib/config"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/crypto/abstract"
	"time"
	"os"
	"io/ioutil"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/nist"
	"log"
	"github.com/dedis/cothority/lib/app"
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
	if app.RunFlags.Mode == "client"{
		app.RunFlags.Hostname = app.RunFlags.Name
	}
	hostname := app.RunFlags.Hostname
	dbg.Lvl3(app.RunFlags.Hostname, "Starting to run")
	if conf.Debug > 1 {
		sign.DEBUG = true
	}

	if hostname == "" {
		log.Fatal("no hostname given", hostname)
	}

	// load the configuration
	dbg.Lvl3("loading configuration for", hostname)
	var hc *config.HostConfig
	var err error
	s := GetSuite(conf.Suite)
	opts := config.ConfigOptions{ConnType: "tcp", Host: hostname, Suite: s}
	if conf.Failures > 0 || conf.FFail > 0 {
		opts.Faulty = true
	}
	hc, err = config.LoadConfig(conf, opts)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	// Wait for everybody to be ready before going on
	ioutil.WriteFile("coll_stamp_up/up" + hostname, []byte("started"), 0666)
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

	defer func() {
		dbg.Lvl1("Collective Signing", hostname, "has terminated in mode", app.RunFlags.Mode)
	}()

	switch app.RunFlags.Mode {
	case "client":
		log.Panic("No client mode")
	case "server":
		RunServer(conf, hc)
	}
}

func GetSuite(suite string) abstract.Suite {
	var s abstract.Suite
	switch {
	case suite == "nist256":
		s = nist.NewAES128SHA256P256()
	case suite == "nist512":
		s = nist.NewAES128SHA256QR512()
	case suite == "ed25519":
		s = ed25519.NewAES128SHA256Ed25519(true)
	default:
		s = nist.NewAES128SHA256P256()
	}
	return s
}

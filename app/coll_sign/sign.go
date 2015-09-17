package coll_sign

import (
	"github.com/dedis/cothority/deploy"
	"github.com/dedis/cothority/proto/sign"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"fmt"
	"log"
	"github.com/dedis/cothority/lib/config"
	"github.com/dedis/crypto/nist"
"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/abstract"
)

// Dispatch-function for running either client or server (mode-parameter)
func Run(mode string, hostname string, conf *deploy.Config) {
	// Do some common setup
	dbg.Lvl1(hostname, "Starting to run")
	if conf.Debug > 1 {
		sign.DEBUG = true
	}

	// fmt.Println("EXEC TIMESTAMPER: " + hostname)
	if hostname == "" {
		fmt.Println("hostname is empty")
		log.Fatal("no hostname given")
	}

	// load the configuration
	//dbg.Lvl2("loading configuration")
	var hc *config.HostConfig
	var err error
	s := GetSuite(conf.Suite)
	opts := config.ConfigOptions{ConnType: "tcp", Host: hostname, Suite: s}
	if conf.Failures > 0 || conf.FFail > 0 {
		opts.Faulty = true
	}
	hc, err = config.LoadConfig("tree.json", opts)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

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

	// run this specific host
	err = hc.Run(true, sign.MerkleTree, hostname)
	if err != nil {
		log.Fatal(err)
	}

	defer func(sn *sign.Node) {
		dbg.Lvl1("Collective Signing", hostname, "has terminated in mode", mode)
		sn.Close()
	}(hc.SNodes[0])

	switch mode{
	case "client":
		RunClient(conf, hc)
	case "server":
		RunServer(conf)
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

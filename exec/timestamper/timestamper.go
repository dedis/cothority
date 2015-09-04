package timestamper

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/ineiti/cothorities/sign"
	"github.com/ineiti/cothorities/helpers/oldconfig"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/nist"
	"github.com/ineiti/cothorities/helpers/logutils"
)

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

func Run(hostname, cfg, app string, rounds int, rootwait int, debug, testConnect bool, failureRate, rFail, fFail int, logger, suite string) {
	if debug {
		//coco.DEBUG = true
	}

	// fmt.Println("EXEC TIMESTAMPER: " + hostname)
	if hostname == "" {
		fmt.Println("hostname is empty")
		log.Fatal("no hostname given")
	}

	// load the configuration
	//log.Println("loading configuration")
	var hc *oldconfig.HostConfig
	var err error
	s := GetSuite(suite)
	opts := oldconfig.ConfigOptions{ConnType: "tcp", Host: hostname, Suite: s}
	if failureRate > 0 || fFail > 0 {
		opts.Faulty = true
	}
	hc, err = oldconfig.LoadConfig(cfg, opts)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	// set FailureRates
	if failureRate > 0 {
		for i := range hc.SNodes {
			hc.SNodes[i].FailureRate = failureRate
		}
	}

	// set root failures
	if rFail > 0 {
		for i := range hc.SNodes {
			hc.SNodes[i].FailAsRootEvery = rFail

		}
	}
	// set follower failures
	// a follower fails on %ffail round with failureRate probability
	for i := range hc.SNodes {
		hc.SNodes[i].FailAsFollowerEvery = fFail
	}

	// run this specific host
	// log.Println("RUNNING HOST CONFIG")
	err = hc.Run(app != "sign", sign.MerkleTree, hostname)
	if err != nil {
		log.Fatal(err)
	}

	defer func(sn *sign.Node) {
		//log.Panicln("program has terminated:", hostname)
		log.Println("Program has terminated:", hostname)
		sn.Close()
	}(hc.SNodes[0])

	if app == "sign" {
		//log.Println("RUNNING Node")
		// if I am root do the announcement message
		if hc.SNodes[0].IsRoot(0) {
			time.Sleep(3 * time.Second)
			start := time.Now()
			iters := 10

			for i := 0; i < iters; i++ {
				time.Sleep(3 * time.Second)
				start = time.Now()
				//fmt.Println("ANNOUNCING")
				hc.SNodes[0].LogTest = []byte("Hello World")
				log.Println("Going to launch announcement ", hc.SNodes[0].Name())
				err = hc.SNodes[0].Announce(0,
					&sign.AnnouncementMessage{
						LogTest: hc.SNodes[0].LogTest,
						Round:   i})
				if err != nil {
					log.Println(err)
				}
				elapsed := time.Since(start)
				log.WithFields(log.Fields{
					"file":  logutils.File(),
					"type":  "root_announced",
					"round": i,
					"time":  elapsed,
				}).Info("")
			}

		} else {
			// otherwise wait a little bit (hopefully it finishes by the end of this)
			time.Sleep(30 * time.Second)
		}
	} else if app == "stamp" || app == "vote" {
		log.Println("RUNNING TIMESTAMPER")
		stampers, _, err := hc.RunTimestamper(0, hostname)
		// get rid of the hc information so it can be GC'ed
		hc = nil
		if err != nil {
			log.Fatal(err)
		}
		for _, s := range stampers {
			// only listen if this is the hostname specified
			if s.Name() == hostname {
				s.Logger = logger
				s.Hostname = hostname
				s.App = app
				if s.IsRoot(0) {
					log.Println("RUNNING ROOT SERVER AT:", hostname, rounds)
					log.Printf("Waiting: %d s\n", rootwait)
					// wait for the other nodes to get set up
					time.Sleep(time.Duration(rootwait) * time.Second)

					log.Println("STARTING ROOT ROUND")
					s.Run("root", rounds)
					// log.Println("\n\nROOT DONE\n\n")

				} else if !testConnect {
					log.Println("RUNNING REGULAR AT:", hostname)
					s.Run("regular", rounds)
					// log.Println("\n\nREGULAR DONE\n\n")
				} else {
					// testing connection
					log.Println("RUNNING TEST_CONNNECT AT:", hostname)
					s.Run("test_connect", rounds)
				}
			}
		}
	}

}

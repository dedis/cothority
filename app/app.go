// usage exec:
//
// exec -name "hostname" -config "tree.json"
//
// -name indicates the name of the node in the tree.json
//
// -config points to the file that holds the configuration.
//     This configuration must be in terms of the final hostnames.
//
// pprof runs on the physical address space [if there is a virtual and physical network layer]
// and if one is specified.

package main

import (
	"flag"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/logutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/deploy"
	"github.com/dedis/cothority/app/coll_sign"
	"github.com/dedis/cothority/app/schnorr_sign"
	"fmt"
	"github.com/dedis/cothority/lib/oldconfig"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/abstract"
	"time"
	"github.com/dedis/cothority/app/coll_stamp"
)


var deter *deploy.Deter
var conf *deploy.Config
var hostname string
var logger string
var physaddr string
var amroot bool
var testConnect bool
var app string
var mode string
var server string
var name string

// TODO: add debug flag for more debugging information (memprofilerate...)
func init() {
	flag.StringVar(&hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.StringVar(&physaddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&amroot, "amroot", false, "am I root node")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&app, "app", app, "Which application to run [coll_sign, coll_stamp]")
	flag.StringVar(&mode, "mode", mode, "Run the app in [server,client] mode")
	flag.StringVar(&name, "name", name, "Name of the node")
	flag.StringVar(&server, "server", "", "the timestamping servers to contact")
}

func main() {
	deter, err := deploy.ReadConfig()
	if err != nil {
		log.Fatal("Couldn't load config-file in exec")
	}
	conf = deter.Config
	dbg.DebugVisible = conf.Debug

	flag.Parse()

	dbg.Lvl1("Running Timestamper", hostname, "with logger", logger)
	defer func() {
		log.Errorln("Terminating host", hostname)
	}()

	// connect with the logging server
	if logger != "" && (amroot || conf.Debug > 0) {
		// blocks until we can connect to the logger
		dbg.Lvl1(hostname, "Connecting to Logger")
		lh, err := logutils.NewLoggerHook(logger, hostname, conf.App)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
		//log.SetOutput(ioutil.Discard)
		//fmt.Println("exiting logger block")
		dbg.Lvl3(hostname, "Done setting up hook")
	}
	if physaddr == "" {
		h, _, err := net.SplitHostPort(hostname)
		if err != nil {
			log.Fatal(hostname, "improperly formatted hostname")
		}
		physaddr = h
	}

	// run an http server to serve the cpu and memory profiles
	go func() {
		_, port, err := net.SplitHostPort(hostname)
		if err != nil {
			log.Fatal(hostname, "improperly formatted hostname: should be host:port")
		}
		p, _ := strconv.Atoi(port)
		// uncomment if more fine grained memory debuggin is needed
		//runtime.MemProfileRate = 1
		dbg.Lvl2(http.ListenAndServe(net.JoinHostPort(physaddr, strconv.Itoa(p + 2)), nil))
	}()

	dbg.Lvl2("Running timestamp with rFail and fFail: ", conf.RFail, conf.FFail)

	switch app{
	case "coll_sign":
		coll_sign.Run(mode, hostname, conf)
	case "coll_stamp":
		if mode == "server" {
			Run(hostname, conf.App, conf.Rounds, conf.RootWait, conf.Debug,
				testConnect, conf.Failures, conf.RFail, conf.FFail, logger, conf.Suite)
		}else {
			coll_stamp.RunClient(logger, server);
		}
	case "schnorr_sign":
		schnorr_sign.Run(mode, conf)
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

func Run(hostname, app string, rounds int, rootwait int, debug int, testConnect bool,
failureRate, rFail, fFail int, logger, suite string) {
	dbg.Lvl1(hostname, "Starting to run")
	if debug > 1 {
		sign.DEBUG = true
	}

	// fmt.Println("EXEC TIMESTAMPER: " + hostname)
	if hostname == "" {
		fmt.Println("hostname is empty")
		log.Fatal("no hostname given")
	}

	// load the configuration
	//dbg.Lvl2("loading configuration")
	var hc *oldconfig.HostConfig
	var err error
	s := GetSuite(suite)
	opts := oldconfig.ConfigOptions{ConnType: "tcp", Host: hostname, Suite: s}
	if failureRate > 0 || fFail > 0 {
		opts.Faulty = true
	}
	hc, err = oldconfig.LoadConfig("tree.json", opts)
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
	// dbg.Lvl3("RUNNING HOST CONFIG")
	err = hc.Run(app != "coll_sign", sign.MerkleTree, hostname)
	if err != nil {
		log.Fatal(err)
	}

	defer func(sn *sign.Node) {
		//log.Panicln("program has terminated:", hostname)
		dbg.Lvl1("Program timestamper has terminated:", hostname)
		sn.Close()
	}(hc.SNodes[0])

	if app == "coll_sign" {
		//dbg.Lvl2("RUNNING Node")
		// if I am root do the announcement message
		if hc.SNodes[0].IsRoot(0) {
			time.Sleep(3 * time.Second)
			start := time.Now()
			iters := 10

			for i := 0; i < iters; i++ {
				//time.Sleep(3 * time.Second)
				start = time.Now()
				//fmt.Println("ANNOUNCING")
				hc.SNodes[0].LogTest = []byte("Hello World")
				dbg.Lvl2("Going to launch announcement ", hc.SNodes[0].Name())
				err = hc.SNodes[0].Announce(0,
					&sign.AnnouncementMessage{
						LogTest: hc.SNodes[0].LogTest,
						Round:   i})
				if err != nil {
					dbg.Lvl1(err)
				}
				elapsed := time.Since(start)
				log.WithFields(log.Fields{
					"file":  logutils.File(),
					"type":  "root_announce",
					"round": i,
					"time":  elapsed,
				}).Info("")
			}

		} else {
			// otherwise wait a little bit (hopefully it finishes by the end of this)
			time.Sleep(30 * time.Second)
		}
	} else if app == "coll_stamp" || app == "vote" {
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
					dbg.Lvl1("Root timestamper at:", hostname, rounds, "Waiting: ", rootwait)
					// wait for the other nodes to get set up
					time.Sleep(time.Duration(rootwait) * time.Second)

					dbg.Lvl1("Starting root-round")
					s.Run("root", rounds)
					// dbg.Lvl2("\n\nROOT DONE\n\n")

				} else if !testConnect {
					dbg.Lvl1("Running regular timestamper on:", hostname)
					s.Run("regular", rounds)
					// dbg.Lvl1("\n\nREGULAR DONE\n\n")
				} else {
					// testing connection
					dbg.Lvl1("Running connection-test on:", hostname)
					s.Run("test_connect", rounds)
				}
			}
		}
	}

}

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

	_ "expvar"

	log "github.com/Sirupsen/logrus"
	"github.com/ineiti/cothorities/deploy/deterlab/exec/timestamper"
	"github.com/ineiti/cothorities/helpers/logutils"
	dbg "github.com/ineiti/cothorities/helpers/debug_lvl"
)

var hostname string
var cfg string
var logger string
var app string
var rounds int
var pprofaddr string
var physaddr string
var rootwait int
var debug int
var failures int
var rFail int
var fFail int
var amroot bool
var testConnect bool
var suite string

// TODO: add debug flag for more debugging information (memprofilerate...)
func init() {
	flag.StringVar(&hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&cfg, "config", "tree.json", "the json configuration file")
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.StringVar(&app, "app", "stamp", "application to run [sign|time]")
	flag.IntVar(&rounds, "rounds", 100, "number of rounds to run")
	flag.StringVar(&pprofaddr, "pprof", ":10000", "the address to run the pprof server at")
	flag.StringVar(&physaddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.IntVar(&rootwait, "rootwait", 30, "the amount of time the root should wait")
	flag.IntVar(&debug, "debug", 2, "set debugging-level")
	flag.IntVar(&failures, "failures", 0, "percent showing per node probability of failure")
	flag.IntVar(&rFail, "rfail", 0, "number of consecutive rounds each root runs before it fails")
	flag.IntVar(&fFail, "ffail", 0, "number of consecutive rounds each follower runs before it fails")
	flag.BoolVar(&amroot, "amroot", false, "am I root node")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&suite, "suite", "nist256", "abstract suite to use [nist256, nist512, ed25519]")
}

func main() {
	flag.Parse()
	dbg.DebugVisible = debug

	dbg.Lvl1("Running Timestamper", hostname, "with logger", logger)
	defer func() {
		log.Errorln("TERMINATING HOST")
	}()

	// connect with the logging server
	if logger != "" && (amroot || debug > 0) {
		// blocks until we can connect to the logger
		dbg.Lvl1(hostname, "Connecting to Logger")
		lh, err := logutils.NewLoggerHook(logger, hostname, app)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("ERROR SETTING UP LOGGING SERVER:", err)
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
		dbg.Lvl2(http.ListenAndServe(net.JoinHostPort(physaddr, strconv.Itoa(p+2)), nil))
	}()

	dbg.Lvl2("Running timestamp with rFail and fFail: ", rFail, fFail)
	timestamper.Run(hostname, cfg, app, rounds, rootwait, debug, testConnect, failures, rFail, fFail, logger, suite)
}

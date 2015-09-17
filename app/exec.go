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
	"github.com/dedis/cothority/lib/logutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/deploy"
	"github.com/dedis/cothority/app/sign"
	"github.com/dedis/cothority/app/schnorr_sign"
	"github.com/dedis/cothority/app/stamp"
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

// TODO: add debug flag for more debugging information (memprofilerate...)
func init() {
	flag.StringVar(&hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&logger, "logger", "", "remote logger")
	flag.StringVar(&physaddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&amroot, "amroot", false, "am I root node")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&app, "app", app, "Which application to run [sign, stamp]")
	flag.StringVar(&mode, "mode", mode, "Run the app in [server,client] mode")

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
	case "sign":
		sign.Run(mode, conf)
	case "stamp":
		Run(hostname, conf.App, conf.Rounds, conf.RootWait, conf.Debug,
			testConnect, conf.Failures, conf.RFail, conf.FFail, logger, conf.Suite)
	case "schnorr_sign":
		schnorr_sign.Run(mode, conf)
	}
}

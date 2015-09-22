// usage exec:
//
// exec -name "appConf.Hostname" -config "tree.json"
//
// -name indicates the name of the node in the tree.json
//
// -config points to the file that holds the configuration.
//     This configuration must be in terms of the final appConf.Hostnames.
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
	"github.com/dedis/cothority/app/coll_stamp"
	"github.com/dedis/cothority/lib/config"
	"os"
)

var deter *deploy.Deter
var conf *deploy.Config
var appConf config.AppConfig

// TODO: add debug flag for more debugging information (memprofilerate...)
func init() {
	flag.StringVar(&appConf.Hostname, "hostname", "", "the appConf.Hostname of this node")
	flag.StringVar(&appConf.Logger, "logger", "", "remote appConf.Logger")
	flag.StringVar(&appConf.PhysAddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&appConf.AmRoot, "amroot", false, "am I root node")
	flag.BoolVar(&appConf.TestConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&appConf.App, "app", appConf.App, "Which application to run [coll_sign, coll_stamp]")
	flag.StringVar(&appConf.Mode, "mode", appConf.Mode, "Run the app in [server,client] mode")
	flag.StringVar(&appConf.Name, "name", appConf.Name, "Name of the node")
	flag.StringVar(&appConf.Server, "server", "", "the timestamping servers to contact")
}

func main() {
	deter, err := deploy.ReadConfig()
	if err != nil {
		log.Fatal("Couldn't load config-file in exec")
	}
	conf = deter.Config
	dbg.DebugVisible = conf.Debug

	flag.Parse()

	dbg.Lvl3("Running", appConf.App, appConf.Hostname, "with logger at", appConf.Logger)
	defer func() {
		log.Errorln("Terminating host", appConf.Hostname)
	}()

	// connect with the logging server
	if appConf.Logger != "" && (appConf.AmRoot || conf.Debug > 0) {
		// blocks until we can connect to the appConf.Logger
		dbg.Lvl3(appConf.Hostname, "Connecting to Logger")
		lh, err := logutils.NewLoggerHook(appConf.Logger, appConf.Hostname, conf.App)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
		//log.SetOutput(ioutil.Discard)
		//fmt.Println("exiting appConf.Logger block")
		dbg.Lvl4(appConf.Hostname, "Done setting up hook")
	}

	if appConf.Mode == "server" {
		if appConf.PhysAddr == "" {
			h, _, err := net.SplitHostPort(appConf.Hostname)
			if err != nil {
				log.Fatal(appConf.Hostname, "improperly formatted hostname", os.Args)
			}
			appConf.PhysAddr = h
		}

		// run an http server to serve the cpu and memory profiles
		go func() {
			_, port, err := net.SplitHostPort(appConf.Hostname)
			if err != nil {
				log.Fatal(appConf.Hostname, "improperly formatted hostname: should be host:port")
			}
			p, _ := strconv.Atoi(port)
			// uncomment if more fine grained memory debuggin is needed
			//runtime.MemProfileRate = 1
			dbg.Lvl3(http.ListenAndServe(net.JoinHostPort(appConf.PhysAddr, strconv.Itoa(p + 2)), nil))
		}()
	}

	dbg.Lvl3("Running timestamp with rFail and fFail: ", conf.RFail, conf.FFail)

	switch appConf.App{
	case "coll_sign":
		coll_sign.Run(&appConf, conf)
	case "coll_stamp":
		coll_stamp.Run(&appConf, conf)
	case "schnorr_sign":
		schnorr_sign.Run(&appConf, conf)
	}
}


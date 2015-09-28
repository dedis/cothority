package app

import (
	"flag"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/logutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/deploy"
	"os"
)

type AppConfig struct {
	Deter   *deploy.Deter
	Conf    *deploy.Config
	Flags	*FlagConfig
}

type FlagConfig struct {
	Hostname    string // Hostname like server-0.cs-dissent ?
	Logger      string // ip addr of the logger to connect to
	PhysAddr    string // physical IP addr of the host
	AmRoot      bool   // is the host root (i.e. special operations)
	TestConnect bool   // Dylan-code to only test the connection and exit afterwards
	Mode        string // ["server", "client"]
	Name        string // Comes from deter.go:187 - "Name of the node"
	Server      string // Timestamping servers to contact
}

var flags FlagConfig

func init() {
	flag.StringVar(&flags.Hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&flags.Logger, "logger", "", "remote logger")
	flag.StringVar(&flags.PhysAddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&flags.AmRoot, "amroot", false, "am I root node")
	flag.BoolVar(&flags.TestConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&flags.Mode, "mode", flags.Mode, "Run the app in [server,client] mode")
	flag.StringVar(&flags.Name, "name", flags.Name, "Name of the node")
	flag.StringVar(&flags.Server, "server", "", "the timestamping servers to contact")
}

func ReadConfig()(*AppConfig) {
	ac := AppConfig{&deploy.Deter{}, &deploy.Config{}, &flags}

	var err error
	err = deploy.ReadConfig(ac.Deter, "deter.toml")
	if err != nil {
		log.Fatal("Couldn't load deter-config-file in exec")
	}
	err = deploy.ReadConfig(ac.Conf, "app.toml")
	if err != nil {
		log.Fatal("Couldn't load app-config-file in exec")
	}
	dbg.DebugVisible = ac.Deter.Debug

	flag.Parse()

	dbg.Lvl3("Running", ac.Deter.App, flags.Hostname, "with logger at", flags.Logger)

	// connect with the logging server
	if flags.Logger != "" && (flags.AmRoot || ac.Deter.Debug > 0) {
		// blocks until we can connect to the flags.Logger
		dbg.Lvl3(flags.Hostname, "Connecting to Logger")
		lh, err := logutils.NewLoggerHook(flags.Logger, flags.Hostname, ac.Deter.App)
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
		//log.SetOutput(ioutil.Discard)
		//fmt.Println("exiting flags.Logger block")
		dbg.Lvl4(flags.Hostname, "Done setting up hook")
	}

	if flags.Mode == "server" {
		if flags.PhysAddr == "" {
			h, _, err := net.SplitHostPort(flags.Hostname)
			if err != nil {
				log.Fatal(flags.Hostname, "improperly formatted hostname", os.Args)
			}
			flags.PhysAddr = h
		}

		// run an http server to serve the cpu and memory profiles
		go func() {
			_, port, err := net.SplitHostPort(flags.Hostname)
			if err != nil {
				log.Fatal(flags.Hostname, "improperly formatted hostname: should be host:port")
			}
			p, _ := strconv.Atoi(port)
			// uncomment if more fine grained memory debuggin is needed
			//runtime.MemProfileRate = 1
			dbg.Lvl3(http.ListenAndServe(net.JoinHostPort(flags.PhysAddr, strconv.Itoa(p + 2)), nil))
		}()
	}
	return &ac
}


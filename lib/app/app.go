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
	"os"
	"reflect"
	"bytes"
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"path/filepath"
)

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

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
var Flags FlagConfig

func init() {
	flag.StringVar(&Flags.Hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&Flags.Logger, "logger", "", "remote logger")
	flag.StringVar(&Flags.PhysAddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&Flags.AmRoot, "amroot", false, "am I root node")
	flag.BoolVar(&Flags.TestConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&Flags.Mode, "mode", Flags.Mode, "Run the app in [server,client] mode")
	flag.StringVar(&Flags.Name, "name", Flags.Name, "Name of the node")
	flag.StringVar(&Flags.Server, "server", "", "the timestamping servers to contact")
}

/*
 * Reads in the config from deterlab and for the application -
 * also parses the init-flags
 * It first reads the configuration of deterlab, in case the
 * application needs any of these configuration-options, then
 * loads over that the configuration of the app.toml
 */
func ReadConfig(conf interface{}, dir ...string) {
	var err error
	err = ReadTomlConfig(conf, "deter.toml", dir...)
	if err != nil {
		dbg.Lvl2("Couldn't load deter-config")
	}
	err = ReadTomlConfig(conf, "app.toml", dir...)
	if err != nil {
		log.Fatal("Couldn't load app-config-file in exec")
	}
	debug := reflect.ValueOf(conf).Elem().FieldByName("Debug")
	if debug.IsValid(){
		dbg.DebugVisible = debug.Interface().(int)
	}

	flag.Parse()

	dbg.LLvl3("Running", Flags.Hostname, "with logger at", Flags.Logger)
	ConnectLogservers()
	ServeMemoryStats()
}

/*
 * Connects to the logservers for external logging
 */
func ConnectLogservers() {
	// connect with the logging server
	if Flags.Logger != "" && Flags.AmRoot{
		// blocks until we can connect to the flags.Logger
		dbg.LLvl3(Flags.Hostname, "Connecting to Logger", Flags.Logger)
		lh, err := logutils.NewLoggerHook(Flags.Logger, Flags.Hostname, "unknown")
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
		//log.SetOutput(ioutil.Discard)
		//fmt.Println("exiting flags.Logger block")
		dbg.Lvl4(Flags.Hostname, "Done setting up hook")
	} else {
		dbg.LLvl4("Not connecting to logger - logger:", Flags.Logger, "AmRoot:", Flags.AmRoot)
	}
}

/*
 * Opens a port at 'flags.Hostname + 1' and serves memory-statistics of this process
 */
func ServeMemoryStats() {
	if Flags.Mode == "server" {
		if Flags.PhysAddr == "" {
			h, _, err := net.SplitHostPort(Flags.Hostname)
			if err != nil {
				log.Fatal(Flags.Hostname, "improperly formatted hostname", os.Args)
			}
			Flags.PhysAddr = h
		}

		// run an http server to serve the cpu and memory profiles
		go func() {
			_, port, err := net.SplitHostPort(Flags.Hostname)
			if err != nil {
				log.Fatal(Flags.Hostname, "improperly formatted hostname: should be host:port")
			}
			p, _ := strconv.Atoi(port)
			// uncomment if more fine grained memory debuggin is needed
			//runtime.MemProfileRate = 1
			res := http.ListenAndServe(net.JoinHostPort(Flags.PhysAddr, strconv.Itoa(p + 2)), nil)
			dbg.Lvl3("Memory-stats server:", res)
		}()
	}
}


/*
 * Writes any structure to a toml-file
 *
 * Takes a filename and an optional directory-name.
 */
func WriteTomlConfig(conf interface{}, filename string, dirOpt ...string) {
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(conf); err != nil {
		dbg.Fatal(err)
	}
	err := ioutil.WriteFile(getFullName(filename, dirOpt...), buf.Bytes(), 0660)
	if err != nil {
		dbg.Fatal(err)
	}
}

/*
 * Reads any structure from a toml-file
 *
 * Takes a filename and an optional directory-name
 */
func ReadTomlConfig(conf interface{}, filename string, dirOpt ...string) (error) {
	buf, err := ioutil.ReadFile(getFullName(filename, dirOpt...))
	if err != nil {
		pwd, _ := os.Getwd()
		dbg.Lvl1("Didn't find", filename, "in", pwd)
		return err
	}

	_, err = toml.Decode(string(buf), conf)
	if err != nil {
		dbg.Fatal(err)
	}

	return nil
}

/*
 * Gets filename and dirname
 *
 * special cases:
 * - filename only
 * - filename in relative path
 * - filename in absolute path
 * - filename and additional path
 */
func getFullName(filename string, dirOpt ...string) string{
	dir := filepath.Dir(filename)
	if len(dirOpt) > 0 {
		dir = dirOpt[0]
	} else {
		if dir == ""{
			dir = "."
		}
	}
	return dir + "/" + filepath.Base(filename)
}
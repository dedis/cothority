package app

import (
	"flag"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"

	"bytes"
	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/logutils"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/nist"
	"strings"
	"syscall"
)

type Flags struct {
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
var RunFlags Flags

func FlagInit() {
	flag.StringVar(&RunFlags.Hostname, "hostname", "", "the hostname of this node")
	flag.StringVar(&RunFlags.Logger, "logger", "", "remote logger")
	flag.StringVar(&RunFlags.PhysAddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&RunFlags.AmRoot, "amroot", false, "am I root node")
	flag.BoolVar(&RunFlags.TestConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&RunFlags.Mode, "mode", RunFlags.Mode, "Run the app in [server,client] mode")
	flag.StringVar(&RunFlags.Name, "name", RunFlags.Name, "Name of the node")
	flag.StringVar(&RunFlags.Server, "server", "", "the timestamping servers to contact")
}

/*
 * Reads in the config for the application -
 * also parses the init-flags
 */
func ReadConfig(conf interface{}, dir ...string) {
	var err error
	err = ReadTomlConfig(conf, "app.toml", dir...)
	if err != nil {
		log.Fatal("Couldn't load app-config-file in exec")
	}
	debug := reflect.ValueOf(conf).Elem().FieldByName("Debug")
	if debug.IsValid() {
		dbg.DebugVisible = debug.Interface().(int)
	}
	FlagInit()
	flag.Parse()

	dbg.Lvl3("Running", RunFlags.Hostname, "with logger at", RunFlags.Logger)
	if RunFlags.AmRoot {
		ConnectLogservers()
	} else {
		dbg.Lvl4("Not connecting to logger - logger:", RunFlags.Logger, "AmRoot:", RunFlags.AmRoot)
	}
	ServeMemoryStats()
}

/*
 * Connects to the logservers for external logging
 */
func ConnectLogservers() {
	// connect with the logging server
	if RunFlags.Logger != "" {
		// blocks until we can connect to the flags.Logger
		dbg.Lvl3(RunFlags.Hostname, "Connecting to Logger", RunFlags.Logger)
		lh, err := logutils.NewLoggerHook(RunFlags.Logger, RunFlags.Hostname, "unknown")
		if err != nil {
			log.WithFields(log.Fields{
				"file": logutils.File(),
			}).Fatalln("Error setting up logging server:", err)
		}
		log.AddHook(lh)
		//log.SetOutput(ioutil.Discard)
		//fmt.Println("exiting flags.Logger block")
		dbg.Lvl4(RunFlags.Hostname, "Done setting up hook")
	} else {
		dbg.Lvl3("Not setting up logserver for", RunFlags.Hostname)
	}
}

/*
 * Opens a port at 'flags.Hostname + 1' and serves memory-statistics of this process
 */
func ServeMemoryStats() {
	if RunFlags.Mode == "server" {
		if RunFlags.PhysAddr == "" {
			h, _, err := net.SplitHostPort(RunFlags.Hostname)
			if err != nil {
				log.Fatal(RunFlags.Hostname, "improperly formatted hostname", os.Args)
			}
			RunFlags.PhysAddr = h
		}

		// run an http server to serve the cpu and memory profiles
		go func() {
			_, port, err := net.SplitHostPort(RunFlags.Hostname)
			if err != nil {
				log.Fatal(RunFlags.Hostname, "improperly formatted hostname: should be host:port")
			}
			p, _ := strconv.Atoi(port)
			// uncomment if more fine grained memory debuggin is needed
			//runtime.MemProfileRate = 1
			res := http.ListenAndServe(net.JoinHostPort(RunFlags.PhysAddr, strconv.Itoa(p+2)), nil)
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
func ReadTomlConfig(conf interface{}, filename string, dirOpt ...string) error {
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
func getFullName(filename string, dirOpt ...string) string {
	dir := filepath.Dir(filename)
	if len(dirOpt) > 0 {
		dir = dirOpt[0]
	} else {
		if dir == "" {
			dir = "."
		}
	}
	return dir + "/" + filepath.Base(filename)
}

// The various suites we can use
var nist256 abstract.Suite = nist.NewAES128SHA256P256()
var nist512 abstract.Suite = nist.NewAES128SHA256QR512()
var edward abstract.Suite = edwards.NewAES128SHA256Ed25519(false)
var nist256Str string = strings.ToLower(nist256.String())
var nist512Str string = strings.ToLower(nist512.String())
var edwardsStr string = strings.ToLower(edward.String())

// Helper functions that will return the suite used during the process from a string name
func GetSuite(suite string) abstract.Suite {
	switch strings.ToLower(suite) {
	case nist256Str: //"nist256", "p256":
		return nist256
	case nist512Str: //"p512":
		return nist512
	case edwardsStr, "ed25519":
		return edward
	default:
		dbg.Lvl1("Got unknown suite", suite)
		return edward
	}
}

func iiToF(sec int64, usec int64) float64 {
	return float64(sec) + float64(usec)/1000000.0
}

// Gets the sytem and the user time so far
func GetRTime() (tSys, tUsr float64) {
	rusage := &syscall.Rusage{}
	syscall.Getrusage(syscall.RUSAGE_SELF, rusage)
	s, u := rusage.Stime, rusage.Utime
	return iiToF(int64(s.Sec), int64(s.Usec)), iiToF(int64(u.Sec), int64(u.Usec))
}

// Returns the difference to the given system- and user-time
func GetDiffRTime(tSys, tUsr float64) (tDiffSys, tDiffUsr float64) {
	nowSys, nowUsr := GetRTime()
	return nowSys - tSys, nowUsr - tUsr
}

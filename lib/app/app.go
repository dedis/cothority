package app

import (
	"flag"
	_ "net/http/pprof"

	"bytes"
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/suites"
	"time"
)

type Flags struct {
	Hostname    string // Hostname like server-0.cs-dissent ?
	Monitor     string // ip addr of the logger to connect to
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
	flag.StringVar(&RunFlags.Monitor, "monitor", "", "remote monitor")
	flag.StringVar(&RunFlags.PhysAddr, "physaddr", "", "the physical address of the noded [for deterlab]")
	flag.BoolVar(&RunFlags.AmRoot, "amroot", false, "am I root node")
	flag.BoolVar(&RunFlags.TestConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&RunFlags.Mode, "mode", RunFlags.Mode, "Run the app in [server,client] mode")
	flag.StringVar(&RunFlags.Name, "name", RunFlags.Name, "Name of the node")
	flag.StringVar(&RunFlags.Server, "server", "", "the timestamping servers to contact")
}

/*
 * Reads in the config for the application -
 * also parses the init-flags and connects to
 * the monitor.
 */
func ReadConfig(conf interface{}, dir ...string) {
	var err error
	err = ReadTomlConfig(conf, "app.toml", dir...)
	if err != nil {
		dbg.Fatal("Couldn't load app-config-file in exec")
	}
	debug := reflect.ValueOf(conf).Elem().FieldByName("Debug")
	if debug.IsValid() {
		dbg.DebugVisible = debug.Interface().(int)
	}
	FlagInit()
	flag.Parse()
	dbg.Lvlf3("Flags are %+v", RunFlags)

	if RunFlags.AmRoot {
		if err := monitor.ConnectSink(RunFlags.Monitor); err != nil {
			dbg.Fatal("Couldn't connect to monitor", err)
		}
	}

	dbg.Lvl3("Running", RunFlags.Hostname, "with monitor at", RunFlags.Monitor)
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

// StartedUp waits for everybody to start by contacting the
// monitor. Argument is total number of peers.
func (f Flags) StartedUp(total int) {
	monitor.Ready(f.Monitor)
	// Wait for everybody to be ready before going on
	for {
		s, err := monitor.GetReady(f.Monitor)
		if err != nil {
			dbg.Lvl1("Couldn't reach monitor:", err)
		} else {
			if s.Ready != total {
				dbg.Lvl4(f.Hostname, "waiting for others to finish", s.Ready, total)
			} else {
				break
			}
		}
		time.Sleep(time.Second)
	}
	dbg.Lvl3(f.Hostname, "thinks everybody's here")
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

// Helper functions that will return the suite used during the process from a string name
func GetSuite(suite string) abstract.Suite {
	s, ok := suites.All()[suite]
	if !ok {
		dbg.Lvl1("Suites available:", suites.All())
		dbg.Fatal("Didn't find suite", suite)
	}
	return s
}

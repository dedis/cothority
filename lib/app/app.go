package app

import (
	"flag"
	_ "net/http/pprof"
	"strings"

	"bytes"
	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/cothority/lib/monitor"
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
var edward abstract.Suite = edwards.NewAES128SHA256Ed25519(true)
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

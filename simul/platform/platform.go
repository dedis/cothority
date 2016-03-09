package platform

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"os"
	"strings"
)

// Generic interface to represent a platform where to run tests
// or direct applications. For now only localhost + deterlab.
// one could imagine EC2 or OpenStack or whatever you can as long as you
// implement this interface !
type Platform interface {
	// Does the initial configuration of all structures needed for the platform
	Configure(*PlatformConfig)
	// Builds all necessary binaries
	Build(string) error
	// Makes sure that there is no part of the application still running
	Cleanup() error
	// Copies the binaries to the appropriate directory/machines, together with
	// the necessary configuration. RunConfig is a simple string that should
	// be copied as 'app.toml' to the directory where the app resides
	Deploy(RunConfig) error
	// Starts the application and returns - non-blocking!
	Start(args ...string) error
	// Waits for the application to quit
	Wait() error
}

// PlatformConfig is passed to Platform.Config and prepares the platform for
// specific system-wide configurations
type PlatformConfig struct {
	MonitorPort int
	Debug       int
}

var deterlab string = "deterlab"
var localhost string = "localhost"
var mininet string = "mininet"

// Return the appropriate platform
// [deterlab,localhost,mininet]
func NewPlatform(t string) Platform {
	switch t {
	case deterlab:
		return &Deterlab{}
	case localhost:
		return &Localhost{}
	case mininet:
		return &MiniNet{}
	}
	return nil
}

/* Reads in a configuration-file for a run. The configuration-file has the
 * following syntax:
 * Name1 = value1
 * Name2 = value2
 * [empty line]
 * n1, n2, n3, n4
 * v11, v12, v13, v14
 * v21, v22, v23, v24
 *
 * The Name1...Namen are global configuration-options.
 * n1..nn are configuration-options for one run
 * Both the global and the run-configuration are copied to both
 * the platform and the app-configuration.
 */
func ReadRunFile(p Platform, filename string) []RunConfig {
	var runconfigs []RunConfig
	masterConfig := NewRunConfig()
	dbg.Lvl3("Reading file", filename)

	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		dbg.Fatal("Couldn't open file", file, err)
	}

	// Decoding of the first part of the run config file
	// where the config wont change for the whole set of the simulation's tests
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		dbg.Lvl3("Decoding", text)
		// end of the first part
		if text == "" {
			break
		}
		if text[0] == '#' {
			continue
		}

		// checking if format is good
		vals := strings.Split(text, "=")
		if len(vals) != 2 {
			dbg.Fatal("Simulation file:", filename, " is not properly formatted ( key = value )")
		}
		// fill in the general config
		masterConfig.Put(strings.TrimSpace(vals[0]), strings.TrimSpace(vals[1]))
		// also put it in platform
		toml.Decode(text, p)
		dbg.Lvlf5("Platform is now %+v", p)
	}

	for {
		scanner.Scan()
		if scanner.Text() != "" {
			break
		}
	}
	args := strings.Split(scanner.Text(), ", ")
	for scanner.Scan() {
		rc := masterConfig.Clone()
		// put each individual test configs
		for i, value := range strings.Split(scanner.Text(), ", ") {
			rc.Put(strings.TrimSpace(args[i]), strings.TrimSpace(value))
		}
		runconfigs = append(runconfigs, *rc)
	}

	return runconfigs
}

// Struct that represent the configuration to apply for one "test"
// Note: a "simulation" is a set of "tests"
type RunConfig struct {
	fields map[string]string
}

func NewRunConfig() *RunConfig {
	rc := new(RunConfig)
	rc.fields = make(map[string]string)
	return rc
}

// One problem for now is RunConfig read also the ' " ' char (34 ASCII)
// and thus when doing Get() , also return the value enclosed by ' " '
// One fix is to each time we Get(), aautomatically delete those chars
var replacer *strings.Replacer = strings.NewReplacer("\"", "", "'", "")

// Returns the associated value of the field in the config
func (r *RunConfig) Get(field string) string {
	return replacer.Replace(r.fields[strings.ToLower(field)])
}

// Insert a new field - value relationship
func (r *RunConfig) Put(field, value string) {
	r.fields[strings.ToLower(field)] = value
}

// Returns this config as bytes in a Toml format
func (r *RunConfig) Toml() []byte {
	var buf bytes.Buffer
	for k, v := range r.fields {
		fmt.Fprintf(&buf, "%s = %s\n", k, v)
	}
	return buf.Bytes()
}

// Returns this config as a Map
func (r *RunConfig) Map() map[string]string {
	tomap := make(map[string]string)
	for k := range r.fields {
		tomap[k] = r.Get(k)
	}
	return tomap
}

// Clone this runconfig so it has all fields-value relationship already present
func (r *RunConfig) Clone() *RunConfig {
	rc := NewRunConfig()
	for k, v := range r.fields {
		rc.fields[k] = v
	}
	return rc
}

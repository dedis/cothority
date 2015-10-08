package platform
import (
	"os"
	"bufio"
	"github.com/BurntSushi/toml"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
"strings"
)

type RunConfig string

type Platform interface {
	// Does the initial configuration of all structures needed for the platform
	Configure()
	// Builds all necessary binaries
	Build(string) error
	// Copies the binaries to the appropriate directory/machines, together with
	// the necessary configuration. RunConfig is a simple string that should
	// be copied as 'app.toml' to the directory where the app resides
	Deploy(RunConfig) error
	// Starts the application and returns - non-blocking!
	Start() error
	// Waits for the application to quit
	Wait() error
	// Stops the application and cleans up eventual other processes
	Stop() error
}

var deterlab string = "deterlab"
var localhost string = "localhost"

// Return the appropriate platform
// [deterlab,localhost]
func NewPlatform(t string) Platform {
	var p Platform
	switch t {
	case deterlab:
		p = &Deterlab{}
	case localhost:
		p = &Localhost{}
	}
	return p
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
func ReadRunFile(p Platform, filename string) []RunConfig{
	var runconfigs []RunConfig

	dbg.Lvl3("Reading file", filename)

	platformString := ""
	file, err := os.Open(filename)
	if err != nil {
		dbg.Fatal("Couldn't open file", file, err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		dbg.Lvl3("Decoding", text)
		if text == "" {
			break
		}
		toml.Decode(text, p)
		platformString += text + "\n"
		dbg.Lvlf3("Platform is now %+v", p)
	}

	scanner.Scan()
	args := strings.Split(scanner.Text(), ", ")
	for scanner.Scan() {
		rc := platformString
		for i, value := range strings.Split(scanner.Text(), ", ") {
			rc += args[i] + " = " + value + "\n"
		}
		runconfigs = append(runconfigs, RunConfig(rc))
	}

	return runconfigs
}
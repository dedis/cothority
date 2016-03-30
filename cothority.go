/*
Cothority-SDA is a framework that allows testing, simulating and
deploying crypto-related protocols.

# Running a simulation

Your best starting point is the simul/-directory where you can start different
protocols and test them on your local computer. After building, you can
start any protocol you like:
	cd simul
	go build
	./simul runfiles/test_cosi.toml
Once a simulation is done, you can look at the results in the test_data-directory:
	cat test_data/test_cosi.csv
To have a simple plot of the round-time, you need to have matplotlib installed
in version 1.5.1.
	matplotlib/plot.py test_data/test_cosi.csv
If plot.py complains about missing matplotlib-library, you can install it using
	sudo easy_install "matplotlib == 1.5.1"
at least on a Mac.

# Writing your own protocol

If you want to experiment with a protocol of your own, have a look at
the protocols-package-documentation.

# Deploying

Unfortunately it's not possible to deploy the Cothority as a stand-alon app.
Well, it is possible, but there is no way to start a protocol.
*/
package main

import (
	"flag"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	// Empty imports to have the init-functions called which should
	// register the protocol

	_ "github.com/dedis/cothority/protocols"
)

/*
Cothority is a general node that can be used for all available protocols.
*/

// ConfigFile represents the configuration for a standalone run
var ConfigFile string
var debugVisible int

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&ConfigFile, "config", "config.toml", "which config-file to use")
	flag.IntVar(&debugVisible, "debug", 1, "verbosity: 0-5")
}

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	dbg.SetDebugVisible(debugVisible)
	// We're in standalone mode and only start the node
	host, err := sda.NewHostFromFile(ConfigFile)
	if err != nil {
		dbg.Fatal("Couldn't get host:", err)
	}
	host.ListenAndWait()
	host.StartProcessMessages()
	host.WaitForClose()
}

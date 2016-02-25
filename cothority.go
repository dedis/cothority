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

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&ConfigFile, "config", "config.toml", "which config-file to use")
	flag.IntVar(&dbg.DebugVisible, "debug", 1, "verbosity: 0-5")
}

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	// We're in standalone mode and only start the node
	host, err := sda.NewHostFromFile(ConfigFile)
	if err != nil {
		dbg.Fatal("Couldn't get host:", err)
	}
	host.Listen()
	go host.ProcessMessages()
	select {
	case <-host.Closed:
	}
}

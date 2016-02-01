package main

import (
	"flag"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	// Empty imports to have the init-functions called which should
	// register the protocol
	"github.com/dedis/cothority/lib/monitor"
	_ "github.com/dedis/cothority/protocols"
	"github.com/dedis/cothority/protocols/manage"
)

/*
Cothority is a general node that can be used for all available protocols.
*/

// The address of this host - if there is only one host in the config
// file, it will be derived from it automatically
var HostAddress string

// ip addr of the logger to connect to
var Monitor string

// Simul is != "" if this node needs to start a simulation of that protocol
var Simul string

// StartProto - if true, this node will 'Start' the protocol
var StartProto bool

// ConfigFile represents the configuration for a standalone run
var ConfigFile string

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&HostAddress, "address", "", "our address to use")
	flag.StringVar(&Simul, "simul", "", "start simulating that protocol")
	flag.BoolVar(&StartProto, "start", false, "whether to start the protocol")
	flag.StringVar(&ConfigFile, "config", "config.toml", "which config-file to use")
	flag.StringVar(&Monitor, "monitor", "", "remote monitor")
	flag.IntVar(&dbg.DebugVisible, "debug", 1, "verbosity: 0-5")
}

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	if Monitor != "" {
		monitor.ConnectSink(Monitor)
	}
	dbg.Lvl3("Flags are:", HostAddress, Simul, StartProto, dbg.DebugVisible)
	if Simul == "" {
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
	} else {
		// There is a protocol to be initialised and perhaps started
		sc, err := sda.LoadSimulationConfig(".", HostAddress)
		if err != nil {
			dbg.Fatal(err)
		}
		sc.Host.Listen()
		go sc.Host.ProcessMessages()
		sim, err := sda.NewSimulation(Simul, sc.Config)
		if err != nil {
			dbg.Fatal(err)
		}
		err = sim.Node(sc)
		if err != nil {
			dbg.Fatal(err)
		}
		if StartProto {
			childrenWait := monitor.NewMeasure("ChildrenWait")
			for {
				dbg.Lvl2("Counting children")
				node, err := sc.Overlay.StartNewNodeName("Count", sc.Tree)
				if err != nil {
					dbg.Fatal(err)
				}
				count := <-node.ProtocolInstance().(*manage.ProtocolCount).Count
				if count == sc.Tree.Size() {
					dbg.Lvl2("Found all children")
					break
				} else {
					dbg.Lvl2("Found only", count, "children")
				}
			}
			childrenWait.Measure()
			dbg.Lvl2("Starting new node", Simul)
			err := sim.Run(sc)
			if err != nil {
				dbg.Fatal(err)
			}
			_, err = sc.Overlay.StartNewNodeName("CloseAll", sc.Tree)
			if err != nil {
				dbg.Fatal(err)
			}
		}
		select {
		case <-sc.Host.Closed:
		}
	}
}

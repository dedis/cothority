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
	"time"
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

// ConfigFile represents the configuration for a standalone run
var ConfigFile string

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&HostAddress, "address", "", "our address to use")
	flag.StringVar(&Simul, "simul", "", "start simulating that protocol")
	flag.StringVar(&ConfigFile, "config", "config.toml", "which config-file to use")
	flag.StringVar(&Monitor, "monitor", "", "remote monitor")
	flag.IntVar(&dbg.DebugVisible, "debug", 1, "verbosity: 0-5")
}

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	dbg.Lvl3("Flags are:", HostAddress, Simul, dbg.DebugVisible, Monitor)
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
		startSimulation()
	}
}

// startSimulation will start all necessary hosts and eventually start the
// simulation.
func startSimulation() {
	// There is a protocol to be initialised and perhaps started
	scs, err := sda.LoadSimulationConfig(".", HostAddress)
	if err != nil {
		// We probably are not needed
		dbg.Lvl2(err)
		return
	}
	if Monitor != "" {
		monitor.ConnectSink(Monitor)
	}
	var sims []sda.Simulation
	var rootSC *sda.SimulationConfig
	var rootSim sda.Simulation
	for _, sc := range scs {
		// Starting all hosts for that server
		host := sc.Host
		dbg.Lvl3("Starting host", host.Entity.Addresses)
		host.Listen()
		go host.ProcessMessages()
		sim, err := sda.NewSimulation(Simul, sc.Config)
		if err != nil {
			dbg.Fatal(err)
		}
		err = sim.Node(sc)
		if err != nil {
			dbg.Fatal(err)
		}
		sims = append(sims, sim)
		if host.Entity.Id == sc.Tree.Root.Entity.Id {
			dbg.Lvl2("Found root-node, will start protocol")
			rootSim = sim
			rootSC = sc
		}
	}
	if rootSim != nil {
		// If this cothority has the root-host, it will start the simulation
		dbg.Lvl2("Starting protocol", Simul, "on host", rootSC.Host.Entity.Addresses)
		//dbg.Lvl5("Tree is", rootSC.Tree.Dump())
		childrenWait := monitor.NewMeasure("ChildrenWait")
		wait := true
		for wait {
			dbg.LLvl2("Counting children")
			node, err := rootSC.Overlay.StartNewNodeName("Count", rootSC.Tree)
			if err != nil {
				dbg.Fatal(err)
			}
			select {
			case count := <-node.ProtocolInstance().(*manage.ProtocolCount).Count:
				if count == rootSC.Tree.Size() {
					dbg.Lvl2("Found all", count, "children")
					wait = false
				} else {
					dbg.Lvl2("Found only", count, "children")
				}
			case <-time.After(time.Second * 10):
				dbg.Lvl1("Timed out waiting for children")
			}
		}
		childrenWait.Measure()
		dbg.Lvl2("Starting new node", Simul)
		err := rootSim.Run(rootSC)
		if err != nil {
			dbg.Fatal(err)
		}
		_, err = rootSC.Overlay.StartNewNodeName("CloseAll", rootSC.Tree)
		if err != nil {
			dbg.Fatal(err)
		}
	} else {
		rootSC = scs[0]
	}

	// Wait for the first host to be closed
	select {
	case <-rootSC.Host.Closed:
		dbg.Lvl2(HostAddress, ": first host closed - quitting")
		monitor.End()
	}
}

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
var hostAddress string

// ip addr of the logger to connect to
var monitorAddress string

// Simul is != "" if this node needs to start a simulation of that protocol
var simul string

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&hostAddress, "address", "", "our address to use")
	flag.StringVar(&simul, "simul", "", "start simulating that protocol")
	flag.StringVar(&monitorAddress, "monitor", "", "remote monitor")
	flag.IntVar(&dbg.DebugVisible, "debug", 1, "verbosity: 0-5")
}

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	dbg.Lvl3("Flags are:", hostAddress, simul, dbg.DebugVisible, monitorAddress)

	scs, err := sda.LoadSimulationConfig(".", hostAddress)
	if err != nil {
		// We probably are not needed
		dbg.Lvl2(err)
		return
	}
	if monitorAddress != "" {
		monitor.ConnectSink(monitorAddress)
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
		sim, err := sda.NewSimulation(simul, sc.Config)
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
		dbg.Lvl2("Starting protocol", simul, "on host", rootSC.Host.Entity.Addresses)
		//dbg.Lvl5("Tree is", rootSC.Tree.Dump())
		childrenWait := monitor.NewMeasure("ChildrenWait")
		wait := true
		for wait {
			dbg.Lvl2("Counting children")
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
					dbg.Lvl1("Found only", count, "children, counting again")
				}
			case <-time.After(time.Second * 10):
				dbg.Lvl1("Timed out waiting for children")
			}
		}
		childrenWait.Measure()
		dbg.Lvl1("Starting new node", simul)
		err := rootSim.Run(rootSC)
		if err != nil {
			dbg.Fatal(err)
		}

		// In case of "SingleHost" we need a new tree that contains every
		// entity only once, whereas rootSC.Tree will have the same
		// entity at different TreeNodes, which makes it difficult to
		// correctly close everything.
		closeTree := rootSC.EntityList.GenerateBinaryTree()
		rootSC.Overlay.RegisterTree(closeTree)
		_, err = rootSC.Overlay.StartNewNodeName("CloseAll", closeTree)
		if err != nil {
			dbg.Fatal(err)
		}
	} else {
		rootSC = scs[0]
	}

	// Wait for the first host to be closed
	select {
	case <-rootSC.Host.Closed:
		dbg.Lvl2(hostAddress, ": first host closed - quitting")
		monitor.End()
	}
}

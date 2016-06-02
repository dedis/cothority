// The simulation cothority used for all protocols.
// This should not be used stand-alone and is only for
// the simulations. It loads the simulation-file, initialises all
// necessary hosts and starts the simulation on the root-node.
package main

import (
	"flag"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"

	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/protocols/manage"
	// Empty imports to have the init-functions called which should
	// register the protocol
	_ "github.com/dedis/cothority/protocols"
	_ "github.com/dedis/cothority/services"
)

// The address of this host - if there is only one host in the config
// file, it will be derived from it automatically
var hostAddress string

// ip addr of the logger to connect to
var monitorAddress string

// Simul is != "" if this node needs to start a simulation of that protocol
var simul string

var debugVisible int

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&hostAddress, "address", "", "our address to use")
	flag.StringVar(&simul, "simul", "", "start simulating that protocol")
	flag.StringVar(&monitorAddress, "monitor", "", "remote monitor")
	flag.IntVar(&debugVisible, "debug", 1, "verbosity: 0-5")
}

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	dbg.SetDebugVisible(debugVisible)
	dbg.Lvl3("Flags are:", hostAddress, simul, dbg.DebugVisible, monitorAddress)

	scs, err := sda.LoadSimulationConfig(".", hostAddress)
	measures := make([]*monitor.CounterIOMeasure, len(scs))
	if err != nil {
		// We probably are not needed
		dbg.Lvl2(err, hostAddress)
		return
	}
	if monitorAddress != "" {
		if err := monitor.ConnectSink(monitorAddress); err != nil {
			dbg.Error("Couldn't connect monitor to sink:", err)
		}
	}
	sims := make([]sda.Simulation, len(scs))
	var rootSC *sda.SimulationConfig
	var rootSim sda.Simulation
	for i, sc := range scs {
		// Starting all hosts for that server
		host := sc.Host
		measures[i] = monitor.NewCounterIOMeasure("bandwidth", host)
		dbg.Lvl3(hostAddress, "Starting host", host.Entity.Addresses)
		host.Listen()
		host.StartProcessMessages()
		sim, err := sda.NewSimulation(simul, sc.Config)
		if err != nil {
			dbg.Fatal(err)
		}
		err = sim.Node(sc)
		if err != nil {
			dbg.Fatal(err)
		}
		sims[i] = sim
		if host.Entity.ID == sc.Tree.Root.Entity.ID {
			dbg.Lvl2(hostAddress, "is root-node, will start protocol")
			rootSim = sim
			rootSC = sc
		}
	}
	if rootSim != nil {
		// If this cothority has the root-host, it will start the simulation
		dbg.Lvl2("Starting protocol", simul, "on host", rootSC.Host.Entity.Addresses)
		//dbg.Lvl5("Tree is", rootSC.Tree.Dump())

		// First count the number of available children
		childrenWait := monitor.NewTimeMeasure("ChildrenWait")
		wait := true
		// The timeout starts with 1 second, which is the time of response between
		// each level of the tree.
		timeout := 1000
		for wait {
			p, err := rootSC.Overlay.CreateProtocol(rootSC.Tree, "Count")
			if err != nil {
				dbg.Fatal(err)
			}
			proto := p.(*manage.ProtocolCount)
			proto.SetTimeout(timeout)
			proto.Start()
			dbg.Lvl1("Started counting children with timeout of", timeout)
			select {
			case count := <-proto.Count:
				if count == rootSC.Tree.Size() {
					dbg.Lvl1("Found all", count, "children")
					wait = false
				} else {
					dbg.Lvl1("Found only", count, "children, counting again")
				}
			}
			// Double the timeout and try again if not successful.
			timeout *= 2
		}
		childrenWait.Record()
		dbg.Lvl1("Starting new node", simul)
		measureNet := monitor.NewCounterIOMeasure("bandwidth_root", rootSC.Host)
		err := rootSim.Run(rootSC)
		if err != nil {
			dbg.Fatal(err)
		}
		measureNet.Record()

		// Test if all Entities are used in the tree, else we'll run into
		// troubles with CloseAll
		if !rootSC.Tree.UsesList() {
			dbg.Error("The tree doesn't use all Entities from the list!\n" +
				"This means that the CloseAll will fail and the experiment never ends!")
		}
		closeTree := rootSC.Tree
		if rootSC.GetSingleHost() {
			// In case of "SingleHost" we need a new tree that contains every
			// entity only once, whereas rootSC.Tree will have the same
			// entity at different TreeNodes, which makes it difficult to
			// correctly close everything.
			dbg.Lvl2("Making new root-tree for SingleHost config")
			closeTree = rootSC.EntityList.GenerateBinaryTree()
			rootSC.Overlay.RegisterTree(closeTree)
		}
		pi, err := rootSC.Overlay.CreateProtocol(closeTree, "CloseAll")
		pi.Start()
		if err != nil {
			dbg.Fatal(err)
		}
	}

	// Wait for all hosts to be closed
	allClosed := make(chan bool)
	go func() {
		for i, sc := range scs {
			sc.Host.WaitForClose()
			// record the bandwidth
			measures[i].Record()
			dbg.Lvl3(hostAddress, "Simulation closed host", sc.Host.Entity.Addresses, "closed")
		}
		allClosed <- true
	}()
	dbg.Lvl3(hostAddress, scs[0].Host.Entity.First(), "is waiting for all hosts to close")
	<-allClosed
	dbg.Lvl2(hostAddress, "has all hosts closed")
	monitor.EndAndCleanup()
}

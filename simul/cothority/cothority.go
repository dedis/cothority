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
	"math"
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

		// First count the number of available children
		childrenWait := monitor.NewMeasure("ChildrenWait")
		wait := true
		// Sets the network-delay between nodes - this also includes the
		// time to open the connection, which can be long in case of more
		// than 500 hosts per machine.
		networkDelay := 500
		// The timeout starts at twice the networkDelay to give even more
		// time for opening the connections - experimental value (like
		// the speed of light)
		timeout := networkDelay * 2 * int(math.Log2(float64(len(rootSC.EntityList.List))))
		for wait {
			dbg.Lvl2("Counting children with timeout of", timeout)
			node, err := rootSC.Overlay.CreateNewNodeName("Count", rootSC.Tree)
			if err != nil {
				dbg.Fatal(err)
			}
			node.ProtocolInstance().(*manage.ProtocolCount).Timeout = timeout
			node.ProtocolInstance().(*manage.ProtocolCount).NetworkDelay = networkDelay
			node.Start()
			select {
			case count := <-node.ProtocolInstance().(*manage.ProtocolCount).Count:
				if count == rootSC.Tree.Size() {
					dbg.Lvl1("Found all", count, "children")
					wait = false
				} else {
					dbg.Lvl1("Found only", count, "children, counting again")
				}
			case <-time.After(time.Millisecond * time.Duration(timeout) * 2):
				// Wait longer than the root-node before aborting
				dbg.Lvl1("Timed out waiting for children")
			}
			// Double the timeout and try again if not successful.
			timeout *= 2
		}
		childrenWait.Measure()
		dbg.Lvl1("Starting new node", simul)
		err := rootSim.Run(rootSC)
		if err != nil {
			dbg.Fatal(err)
		}

		if rootSC.GetSingleHost() {
			// In case of "SingleHost" we need a new tree that contains every
			// entity only once, whereas rootSC.Tree will have the same
			// entity at different TreeNodes, which makes it difficult to
			// correctly close everything.
			dbg.Lvl2("Making new root-tree for SingleHost config")
			closeTree := rootSC.EntityList.GenerateBinaryTree()
			rootSC.Overlay.RegisterTree(closeTree)
			_, err = rootSC.Overlay.StartNewNodeName("CloseAll", closeTree)
		} else {
			_, err = rootSC.Overlay.StartNewNodeName("CloseAll", rootSC.Tree)
		}
		monitor.End()
		if err != nil {
			dbg.Fatal(err)
		}
	}

	// Wait for all hosts to be closed
	allClosed := make(chan bool, 1)
	go func() {
		for _, sc := range scs {
			<-sc.Host.Closed
			dbg.Lvl4("Host", sc.Host.Entity.Addresses, "closed")
		}
		allClosed <- true
	}()
	select {
	case <-allClosed:
		dbg.Lvl2(hostAddress, ": all hosts closed")
	case <-time.After(time.Second * time.Duration(scs[0].GetCloseWait())):
		dbg.Lvl2(hostAddress, ": didn't close after 2 minutes")
	}
}

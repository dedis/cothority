package main

/*
The simulation-file can be used with the `cothority/simul` and be run either
locally or on deterlab. Contrary to the `test` of the protocol, the simulation
is much more realistic, as it tests the protocol on different nodes, and not
only in a test-environment.

The Setup-method is run once on the client and will create all structures
and slices necessary to the simulation. It also receives a 'dir' argument
of a directory where it can write files. These files will be copied over to
the simulation so that they are available.

The Run-method is called only once by the root-node of the tree defined in
Setup. It should run the simulation in different rounds. It can also
measure the time each run takes.

In the Node-method you can read the files that have been created by the
'Setup'-method.
*/

import (
	//"math/rand"
	//"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/dns_id/sidentity"
	"github.com/dedis/cothority/dns_id/webserver"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	//"gopkg.in/dedis/onet.v1/simul/monitor"
)

func init() {
	onet.SimulationRegister("SkipCert", NewSimulation)
}

// Simulation implements onet.Simulation.
type Simulation struct {
	onet.SimulationBFTree
	CK           int
	WK           int
	Clients      int
	Evol1        int
	Evol2        int
	Updates      int
}

// NewSimulation is used internally to register the simulation (see the init()
// function above).
func NewSimulation(config string) (onet.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation.
func (e *Simulation) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

type UserInfo struct {
	*webserver.User
	name string
}

// Run implements onet.Simulation.
func (e *Simulation) Run(config *onet.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	log.Lvlf2("Roster is: %s", config.Roster)

	s := config.GetService(sidentity.ServiceName).(*sidentity.Service)
	var roster = config.Roster

	s.WaitSetupWkhs(roster, e.Clients, e.CK, e.WK)

	siteInfoList := s.WaitSetup(roster, e.Clients, e.CK, e.WK, e.Evol1, e.Evol2)

	// wait for the 1st timestamper round to complete
	s.WaitTimestamper(roster, e.Clients, e.CK)
	log.Print("--------after 1st timestamper round completes")
	s.WaitWebservers(roster, e.Clients, e.CK, e.Updates)

	//time.Sleep(time.Duration(15*1000) * time.Millisecond)

	doneCh := make(chan bool)
	go func() {
		s.WaitClients(roster, e.Clients, e.CK, e.WK, e.Evol1, e.Evol2, siteInfoList)
		doneCh <- true
	}()

	cnt := 0

	for _ = range doneCh {
		cnt++
		if cnt == 1 {
			close(doneCh)
			break
		}
	}



	//log.Print("SIMULATION FINISHES IN ABOUT 20 SECONDS")
	//time.Sleep(time.Duration(20*1000) * time.Millisecond)
	//log.Print("SIMULATION FINISHED ")
	return nil
}

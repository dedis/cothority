package prifi

import (
	"errors"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

/*
This is a simple ExampleHandlers-protocol with two steps:
- announcement - which sends a message to all children
- reply - used for counting the number of children
*/

func init() {
	sda.SimulationRegister("PriFi", NewSimulation)
}

// Simulation implements sda.Simulation.
type Simulation struct {
	sda.SimulationBFTree
}

// NewSimulation is used internally to register the simulation (see the init()
// function above).
func NewSimulation(config string) (sda.Simulation, error) {

	dbg.Lvl1("PriFi Service received New Protocol event - 2")

	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation.
func (e *Simulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run implements sda.Simulation.
func (e *Simulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	dbg.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		p, err := config.Overlay.CreateProtocol(config.Tree, "PriFi")
		if err != nil {
			return err
		}
		dbg.Print("Protocol created")
		go p.Start()

		children := <-p.(*PriFiSDAWrapper).ChildCount
		round.Record()
		if children != size {
			return errors.New("Didn't get " + strconv.Itoa(size) +
				" children")
		}
	}
	return nil
}

// Node - standard registers the entityList and the Tree with that Overlay,
// so we don't have to pass that around for the experiments.
//func (s *SimulationBFTree) Node(sc *SimulationConfig) error {
//	sc.Overlay.RegisterEntityList(sc.EntityList)
//	sc.Overlay.RegisterTree(sc.Tree)
//	return nil
//}

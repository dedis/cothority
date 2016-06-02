package manage

import (
	"errors"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

/*
Defines the simulation for the count-protocol
*/

func init() {
	sda.SimulationRegister("Count", NewSimulation)
}

// Simulation only holds the BFTree simulation
type simulation struct {
	sda.SimulationBFTree
}

// NewSimulation returns the new simulation, where all fields are
// initialised using the config-file
func NewSimulation(config string) (sda.Simulation, error) {
	es := &simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (e *simulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run is used on the destination machines and runs a number of
// rounds
func (e *simulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	dbg.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		p, err := config.Overlay.CreateProtocol(config.Tree, "Count")
		if err != nil {
			return err
		}
		go p.Start()
		children := <-p.(*ProtocolCount).Count
		round.Record()
		if children != size {
			return errors.New("Didn't get " + strconv.Itoa(size) +
				" children")
		}
	}
	return nil
}

package swupdate

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
)

/*
 * Defines the simulation for the service-template to be run with
 * `cothority/simul`.
 */

func init() {
	sda.SimulationRegister("SwUpClient", NewClientSimulation)
}

// Simulation only holds the BFTree simulation
type clientSimulation struct {
	sda.SimulationBFTree
}

// NewSimulation returns the new simulation, where all fields are
// initialised using the config-file
func NewClientSimulation(config string) (sda.Simulation, error) {
	es := &clientSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (e *clientSimulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run is used on the destination machines and runs a number of
// rounds
func (e *clientSimulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
}

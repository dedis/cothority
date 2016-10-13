package proto

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/sda"
)

func init() {
	sda.SimulationRegister("RandHoundCoRound", NewRHSimulation)
}

// RHSimulation implements a RandHound simulation
type RHSimulation struct {
	sda.SimulationBFTree
	Shards uint32
}

// NewRHSimulation creates a new RandHound simulation
func NewRHSimulation(config string) (sda.Simulation, error) {
	rhs := new(RHSimulation)
	_, err := toml.Decode(config, rhs)
	if err != nil {
		return nil, err
	}
	return rhs, nil
}

// Setup configures a RandHound simulation with certain parameters
func (rhs *RHSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	rhs.Hosts = len(hosts)
	rhs.CreateRoster(sim, hosts, 2000)
	err := rhs.CreateTree(sim)
	if err != nil {
		return nil, err
	}
	return sim, nil
}

// Run initiates a RandHound simulation
func (rhs *RHSimulation) Run(config *sda.SimulationConfig) error {

}

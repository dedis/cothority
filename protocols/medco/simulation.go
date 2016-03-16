package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/BurntSushi/toml"
)

func init() {
	sda.SimulationRegister("PrivateCount", NewPrivateCountSimulation)
}

type PrivateCountSimulation struct {
	sda.SimulationBFTree
}

func NewPrivateCountSimulation(config string) (sda.Simulation, error) {
	sim := &PrivateCountSimulation{}
	_,err := toml.Decode(config, sim)
	if err != nil {
		return nil,err
	}
	return sim, nil
}

func (sim *PrivateCountSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
return nil,nil
}

func (sim *PrivateCountSimulation) Node(config *sda.SimulationConfig) error {
return nil
}

func (sim *PrivateCountSimulation) Run(config *sda.SimulationConfig) error {
return nil
}
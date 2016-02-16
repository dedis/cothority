package pbft

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("PbftSimulation", NewSimulation)
	sda.ProtocolRegisterName("BizCoin", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewProtocol(n) })
}

// Simulation implements sda.Simulation interface
type Simulation struct {
	// sda fields:
	sda.SimulationBFTree
	// pbft simulation specific fields:
	// TODO
}

func NewSimulation(config string) (sda.Simulation, error) {
	sim := &Simulation{}
	_, err := toml.Decode(config, sim)
	if err != nil {
		return nil, err
	}
	return sim, nil
}

// Setup implements sda.Simulation interface
func (e *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	// TODO
	return nil
}

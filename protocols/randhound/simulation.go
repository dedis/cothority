package randhound

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("RandHound", NewRHSimulation)
}

type RHSimulation struct {
	sda.SimulationBFTree
	T       int
	R       int
	N       int
	Purpose string
}

func NewRHSimulation(config string) (sda.Simulation, error) {
	rhs := new(RHSimulation)
	_, err := toml.Decode(config, rhs)
	if err != nil {
		return nil, err
	}
	return rhs, nil
}

func (rh *RHSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	rh.CreateEntityList(sim, hosts, 2000)
	err := rh.CreateTree(sim)
	return sim, err
}

func (rh *RHSimulation) Run(config *sda.SimulationConfig) error {
	//size := config.Tree.Size()
	//msg := []byte("Test message for RandHound simulation")
	node, err := config.Overlay.CreateNewNodeName("RandHound", config.Tree)
	if err != nil {
		return err
	}
	proto := node.ProtocolInstance().(*RandHound)
	proto.Start()
	return nil

}

package randhound

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/sda"
)

const protocolName string = "ProtocolRandHound"

func init() {
	sda.SimulationRegister("RHSimulation", NewRHSimulation)
	var T, R, N int = 3, 3, 5           // TODO: extract those values from the .toml file
	var p string = "RandHound test run" // TODO: extract those values from the .toml file
	fn := func(node *sda.Node) (sda.ProtocolInstance, error) {
		return NewRandHound(node, T, R, N, p)
	}
	sda.ProtocolRegisterName(protocolName, fn)
}

type RHSimulation struct {
	sda.SimulationBFTree
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
	node, err := config.Overlay.CreateNewNodeName(protocolName, config.Tree)
	if err != nil {
		return err
	}
	proto := node.ProtocolInstance().(*RandHound)
	proto.Start()
	return nil

}

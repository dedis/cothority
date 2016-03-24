package randhound

import (
	"log"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("RandHound", NewRHSimulation)
}

type RHSimulation struct {
	sda.SimulationBFTree
	Hosts   int
	K       int
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
	node, err := config.Overlay.CreateNewNodeName("RandHound", config.Tree)
	if err != nil {
		return err
	}
	proto := node.ProtocolInstance().(*RandHound)
	proto.Setup(rh.Purpose, rh.Hosts, rh.K)
	proto.Start()

	bytes := make([]byte, 32)
	select {
	case _ = <-proto.Done:
		log.Printf("RandHound - done")
		bytes = <-proto.Result
	case <-time.After(time.Second * 60):
		log.Printf("RandHound - time out")
	}
	log.Printf("RandHound - random bytes: %v\n", bytes)

	return nil

}

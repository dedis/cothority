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
	Trustees int
	Purpose  string
}

func NewRHSimulation(config string) (sda.Simulation, error) {
	rhs := new(RHSimulation)
	_, err := toml.Decode(config, rhs)
	if err != nil {
		return nil, err
	}
	return rhs, nil
}

func (rhs *RHSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	rhs.Hosts = len(hosts)
	rhs.CreateEntityList(sim, hosts, 2000)
	err := rhs.CreateTree(sim)
	if err != nil {
		return nil, err
	}
	return sim, nil
}

func (rhs *RHSimulation) Run(config *sda.SimulationConfig) error {
	leader, err := config.Overlay.CreateNewNodeName("RandHound", config.Tree)
	if err != nil {
		return err
	}
	rh := leader.ProtocolInstance().(*RandHound)
	err = rh.Setup(rhs.Hosts, rhs.Trustees, rhs.Purpose)
	if err != nil {
		return err
	}
	rh.StartProtocol()

	bytes := make([]byte, 32)
	select {
	case _ = <-rh.Leader.Done:
		log.Printf("RandHound - done")
		bytes = <-rh.Leader.Result
	case <-time.After(time.Second * 60):
		log.Printf("RandHound - time out")
	}
	log.Printf("RandHound - random bytes: %v\n", bytes)

	return nil

}

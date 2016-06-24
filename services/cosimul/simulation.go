package cosimul

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cosi/service"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/cosi"
)

func init() {
	sda.SimulationRegister("CosiService", NewSimulation)
}

// Simulation implements the sda.Simulation of the CoSi protocol.
type Simulation struct {
	sda.SimulationBFTree
}

// NewSimulation returns an sda.Simulation or an error if sth. is wrong.
// Used to register the CoSi protocol.
func NewSimulation(config string) (sda.Simulation, error) {
	cs := &Simulation{}
	_, err := toml.Decode(config, cs)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

// Setup implements sda.Simulation.
func (cs *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	cs.CreateRoster(sim, hosts, 2000)
	err := cs.CreateTree(sim)
	return sim, err
}

// Node implements sda.Simulation.
func (cs *Simulation) Node(sc *sda.SimulationConfig) error {
	err := cs.SimulationBFTree.Node(sc)
	if err != nil {
		return err
	}
	return nil
}

// Run implements sda.Simulation.
func (cs *Simulation) Run(config *sda.SimulationConfig) error {
	size := len(config.Roster.List)
	msg := []byte("Hello World Cosi Simulation")
	log.Lvl2("Simulation starting with: Size=", size, ", Rounds=", cs.Rounds)
	for round := 0; round < cs.Rounds; round++ {
		log.Lvl1("Starting round", round)
		roundM := monitor.NewTimeMeasure("round")
		client := service.NewClient()
		resp, err := client.SignMsg(config.Roster, msg)

		if err != nil {
			log.Error("Error while asking for signature:", err)
		}

		if err = cosi.VerifySignature(config.Host.Suite(), config.Roster.Publics(),
			msg, resp.Signature); err != nil {
			log.Error("Invalid signature !")
		}
		roundM.Record()
	}
	log.Lvl1("Simulation finished")
	return nil
}

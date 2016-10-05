package template

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/sda"
)

/*
 * Defines the simulation for the service-template to be run with
 * `cothority/simul`.
 */

func init() {
	sda.SimulationRegister("ServiceTemplate", NewSimulation)
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
	e.CreateRoster(sc, hosts, 2000)
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
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		log.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		service, ok := config.GetService(ServiceName).(*Service)
		if service == nil || !ok {
			log.Fatal("Didn't find service", ServiceName)
		}
		ret, err := service.ClockRequest(nil, &ClockRequest{Roster: config.Roster})
		if err != nil {
			log.Error(err)
		}
		resp, ok := ret.(*ClockResponse)
		if resp.Time <= 0 {
			log.Error("0 time elapsed")
		}
		round.Record()
	}
	return nil
}

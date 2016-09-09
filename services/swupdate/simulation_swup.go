package swupdate

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
	sda.SimulationRegister("SwUpCreate", NewCreateSimulation)
}

// Simulation only holds the BFTree simulation
type createSimulation struct {
	sda.SimulationBFTree
	Height int
	Base   int
}

// NewSimulation returns the new simulation, where all fields are
// initialised using the config-file
func NewCreateSimulation(config string) (sda.Simulation, error) {
	es := &createSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (e *createSimulation) Setup(dir string, hosts []string) (
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
func (e *createSimulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	//var packages []string
	var csvLines []string
	service, ok := config.GetService(ServiceName).(*Service)
	if service == nil || !ok {
		log.Fatal("Didn't find service", ServiceName)
	}
	for _, line := range csvLines {
		log.Lvl1("Parsing line", line)
		// Verify if it's the first version of that packet
		var isFirstPacket bool
		// Fetch the policy-file and the signatures
		var lineFile string
		policy, err := NewPolicy(lineFile)
		log.ErrFatal(err)
		round := monitor.NewTimeMeasure("build_" + policy.Name)
		if isFirstPacket {
			// Create the skipchain, will build
			service.CreatePackage(nil,
				&CreatePackage{
					Roster: config.Roster,
					Base:   e.Base,
					Height: e.Height,
				})
		} else {
			// Append to skipchain, will build
			service.UpdatePackage(nil,
				&UpdatePackage{})
		}
		round.Record()
	}
	return nil
}

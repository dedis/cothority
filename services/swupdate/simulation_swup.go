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
	service, ok := config.GetService(ServiceName).(*Service)
	if service == nil || !ok {
		log.Fatal("Didn't find service", ServiceName)
	}

	packets := make(map[string]*SwupChain)
	drs, err := GetReleases("../../../services/swupdate/snapshot/updates.csv")
	if err != nil {
		return err
	}
	for _, dr := range drs {
		pol := dr.Policy
		log.Lvl1("Building", pol.Name, pol.Version)
		// Verify if it's the first version of that packet
		sc, knownPacket := packets[pol.Name]
		release := &Release{pol, dr.Signatures}
		round := monitor.NewTimeMeasure("full_" + pol.Name)
		if knownPacket {
			// Append to skipchain, will build
			service.UpdatePackage(nil,
				&UpdatePackage{sc, release})
		} else {
			// Create the skipchain, will build
			cp, err := service.CreatePackage(nil,
				&CreatePackage{
					Roster:  config.Roster,
					Base:    e.Base,
					Height:  e.Height,
					Release: release})
			if err != nil {
				return err
			}
			packets[pol.Name] = cp.(*CreatePackageRet).SwupChain
		}
		round.Record()
	}
	return nil
}

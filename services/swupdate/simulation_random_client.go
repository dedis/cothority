package swupdate

import (
	"time"

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
	sda.SimulationRegister("SwUpRandClient", NewRandClientSimulation)
}

// Simulation only holds the BFTree simulation
type randClientSimulation struct {
	sda.SimulationBFTree
	// How many days between two updates
	Frequency int
	Base      int
	Height    int
}

// NewSimulation returns the new simulation, where all fields are
// initialised using the config-file
func NewRandClientSimulation(config string) (sda.Simulation, error) {
	es := &randClientSimulation{Base: 2, Height: 10}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (e *randClientSimulation) Setup(dir string, hosts []string) (
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
func (e *randClientSimulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	service, ok := config.GetService(ServiceName).(*Service)
	if service == nil || !ok {
		log.Fatal("Didn't find service", ServiceName)
	}
	packets := make(map[string]*SwupChain)
	drs, err := GetReleases("../../../services/swupdate/snapshot/updates.csv")
	if err != nil {
		return err
	}
	now := drs[0].Time
	for _, dr := range drs {
		if dr.Time.Sub(now) >= time.Duration(e.Frequency)*time.Hour*24 {
			// Measure bandwidth-usage for updating client
			now = dr.Time
		}

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

package swupdate

import (
	"sync"

	"crypto/rand"
	"crypto/sha256"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/timestamp"
	"gopkg.in/dedis/cothority.v0/lib/monitor"
)

/*
 * Defines the simulation for the service-template to be run with
 * `cothority/simul`.
 */

func init() {
	sda.SimulationRegister("SwUpFlood", NewFloodSimulation)
}

// Simulation only holds the BFTree simulation
type floodSimulation struct {
	sda.SimulationBFTree
	Requests int
}

// NewSimulation returns the new simulation, where all fields are
// initialised using the config-file
func NewFloodSimulation(config string) (sda.Simulation, error) {
	es := &floodSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (e *floodSimulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	c := timestamp.NewClient()
	// TODO move all params to config file:
	maxIterations := 100
	_, err = c.SetupStamper(sc.Roster, time.Millisecond*50, maxIterations)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run is used on the destination machines and runs a number of
// rounds
func (e *floodSimulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	service, ok := config.GetService(ServiceName).(*Service)
	if service == nil || !ok {
		log.Fatal("Didn't find service", ServiceName)
	}
	// Get all packages
	packages := InitializePackages("", nil, 2, 10)
	// Make a DOS-measurement of what the services can handle
	pscRaw, err := service.PackageSC(nil, &PackageSC{packages[0]})
	log.ErrFatal(err)
	psc := pscRaw.(*PackageSCRet)
	wg := sync.WaitGroup{}
	timeClient := timestamp.NewClient()
	m := monitor.NewTimeMeasure("update_empty")
	for req := 0; req < e.Requests; req++ {
		wg.Add(1)
		go func() {
			// Request to the swupchain.
			res, err := service.LatestBlock(nil, &LatestBlock{psc.Last.Hash})
			log.ErrFatal(err)
			lbret, ok := res.(*LatestBlockRet)
			if !ok {
				log.Fatal("Got invalid response.")
			}

			// Get Timestamp from timestamper.
			r := make([]byte, 20)
			_, err = rand.Read(r)
			log.ErrFatal(err, "Couldn't read random bytes:")
			nonce := sha256.Sum256(r)
			root := config.Roster.List[0]
			resp, err := timeClient.SignMsg(root, nonce[:])
			log.ErrFatal(err, "Couldn't sign nonce.")

			// Verify the time is in the good range.
			ts := time.Unix(resp.Timestamp, 0)
			latesBlockTime := time.Unix(lbret.Timestamp.Timestamp, 0)
			if ts.Sub(latesBlockTime) > time.Hour {
				log.Warn("Timestamp of latest block is older than one hour!")
			}

			wg.Done()
		}()
	}
	wg.Wait()
	m.Record()

	// Measure how long it takes to update from the first to the latest block.
	m = monitor.NewTimeMeasure("update_full")
	for req := 0; req < e.Requests; req++ {
		wg.Add(1)
		go func() {
			// Request to the swupchain.
			res, err := service.LatestBlock(nil, &LatestBlock{psc.First.Hash})
			log.ErrFatal(err)
			lbret, ok := res.(*LatestBlockRet)
			if !ok {
				log.Fatal("Got invalid response.")
			}

			// Get Timestamp from timestamper.
			r := make([]byte, 20)
			_, err = rand.Read(r)
			log.ErrFatal(err, "Couldn't read random bytes:")
			nonce := sha256.Sum256(r)
			root := config.Roster.List[0]
			resp, err := timeClient.SignMsg(root, nonce[:])
			log.ErrFatal(err, "Couldn't sign nonce.")

			// Verify the time is in the good range.
			ts := time.Unix(resp.Timestamp, 0)
			latesBlockTime := time.Unix(lbret.Timestamp.Timestamp, 0)
			if ts.Sub(latesBlockTime) > time.Hour {
				log.Warn("Timestamp of latest block is older than one hour!")
			}

			wg.Done()
		}()
	}
	wg.Wait()
	m.Record()
	return nil
}

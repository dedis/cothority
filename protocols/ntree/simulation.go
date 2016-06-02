package ntree

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("NaiveTree", NewSimulation)
}

// Simulation holds everything necessary for one NTree-round
type Simulation struct {
	sda.SimulationBFTree
	Message string
	// 0 - no check
	// 1 - check only direct children's signature
	// 2 - check the whole subtree
	Checking int
}

// NewSimulation creates a new NTree-simulation
func NewSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{Checking: 2}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup prepares the simulation on the local end
func (e *Simulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run starts the simulation on the simulation-side
func (e *Simulation) Run(config *sda.SimulationConfig) error {
	msg := []byte(e.Message)
	size := config.Tree.Size()
	dbg.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round, "with message", string(msg))
		round := monitor.NewTimeMeasure("round")

		p, err := config.Overlay.CreateProtocol(config.Tree, "NaiveTree")
		if err != nil {
			dbg.Error("Quitting the simulation....", err)
			return err
		}
		pi := p.(*Protocol)
		pi.Message = msg
		pi.verifySignature = e.Checking

		done := make(chan bool)
		pi.TreeNodeInstance.OnDoneCallback(func() bool {
			done <- true
			return true
		})
		err = pi.Start()
		if err != nil {
			dbg.Error("Quitting the simulation....", err)
			return err
		}
		<-done
		round.Record()
	}
	return nil
}

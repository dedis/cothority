package bizcoin

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("ExampleChannels", NewBizCoinSimulation)
}

type BizCoinSimulation struct {
	sda.SimulationBFTree
}

func NewBizCoinSimulation(config string) (sda.Simulation, error) {
	es := &BizCoinSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

func (e *BizCoinSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	// TODO will the tree be re-created / broadcasted in every round? (in Run())
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (e *BizCoinSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		round := monitor.NewMeasure("round_XXX")
		// TODO what exactly to measure?
		round.Measure()
	}
	return nil
}

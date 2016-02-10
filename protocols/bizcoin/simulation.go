package bizcoin

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("BizCoinSimulation", NewBizCoinSimulation)
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
	dbg.Lvl1("Simulation starting with: Size=", size, ", Rounds=", cs.Rounds)
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		round := monitor.NewMeasure("round")
		// create the node with the protocol, but do NOT start it yet.
		node, err := config.Overlay.CreateNewNodeName("ProtocolCosi", config.Tree)
		if err != nil {
			return err
		}
		// the protocol itself
		proto := node.ProtocolInstance().(*BizCoin)

		round.Measure()
	}
	return nil
}

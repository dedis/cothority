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
	// sda fields:
	sda.SimulationBFTree
	// your simulation specific fields:
	SimulationConfig
}

type SimulationConfig struct {
	// block-size in bytes:
	Blocksize int
	// number of transactions the client will send:
	NumClientTxs int
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
	server := NewServer(e.BlockSize)
	// TODO create "server" and "client"
	for round := 0; round < e.Rounds; round++ {

		dbg.Lvl1("Starting round", round)
		// create an empty node
		node, err := config.Overlay.CreateNodeEmpty("BizCoin", config.Tree)
		if err != nil {
			return err
		}

		// instantiate a bizcoin protocol
		pi, err := server.Instantiate(node)
		if err != nil {
			return err
		}

		round := monitor.NewMeasure("round")

		round.Measure()
	}
	return nil
}

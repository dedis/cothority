package bizcoin

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("BizCoinSimulation", NewSimulation)
	sda.ProtocolRegisterName("BizCoin", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewBizCoinProtocol(n) })
}

// Simulation implements da.Simulation interface
type Simulation struct {
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
	//blocksDir is the directory where to find the transaction blocks (.dat files)
	BlocksDir string
}

func NewSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation interface
func (e *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run implements sda.Simulation interface
func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	dbg.Lvl1("Simulation starting with:  Rounds=", e.Rounds)
	server := NewServer(e.Blocksize)
	client := NewClient(server)
	go client.StartClientSimulation(e.BlocksDir, e.NumClientTxs)
	sigChan := server.BlockSignaturesChan()
	var rChallComm *monitor.Measure
	var rRespPrep *monitor.Measure
	for round := 0; round < e.Rounds; round++ {

		dbg.Lvl1("Starting round", round)
		// create an empty node
		node, err := sdaConf.Overlay.NewNodeEmptyName("BizCoin", sdaConf.Tree)
		if err != nil {
			return err
		}

		// instantiate a bizcoin protocol
		rComplete := monitor.NewMeasure("round_prepare")
		pi, err := server.Instantiate(node)
		if err != nil {
			return err
		}
		bz := pi.(*BizCoin)
		bz.OnChallengeCommStart(func() {
			rChallComm = monitor.NewMeasure("round_challenge_commit")
		})
		bz.OnChallengeCommFinish(func() {
			rChallComm.Measure()
			rChallComm = nil
		})
		bz.OnResponsePrepareStart(func() {
			rRespPrep = monitor.NewMeasure("round_hanle_resp_prep")
		})
		bz.OnResponsePrepareFinish(func() {
			rRespPrep.Measure()
			rRespPrep = nil
		})

		// wait for the signature (all steps finished)
		<-sigChan
		rComplete.Measure()
	}
	return nil
}

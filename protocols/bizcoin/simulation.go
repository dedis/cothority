package bizcoin

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.SimulationRegister("BizCoinSimulation", NewBizCoinSimulation)
	sda.ProtocolRegisterName("BizCoin", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewBizCoinProtocol(n) })
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
	//blocksDir is the directory where to find the transaction blocks (.dat files)
	BlocksDir string
}

func NewBizCoinSimulation(config string) (sda.Simulation, error) {
	es := &BizCoinSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation interface
func (e *BizCoinSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run implements sda.Simulation interface
func (e *BizCoinSimulation) Run(sdaConf *sda.SimulationConfig) error {
	dbg.Lvl1("Simulation starting with:  Rounds=", e.Rounds)
	server := NewServer(e.Blocksize)
	client := NewClient(server)
	go client.StartClientSimulation(e.BlocksDir, e.NumClientTxs)
	sigChan := server.BlockSignaturesChan()
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		// create an empty node
		node, err := sdaConf.Overlay.NewNodeEmptyName("BizCoin", sdaConf.Tree)
		if err != nil {
			return err
		}
		// instantiate a bizcoin protocol
		rPrepare := monitor.NewMeasure("round_prepare")
		_, err = server.Instantiate(node)
		if err != nil {
			return err
		}
		dbg.Print("after instantiate")
		// wait for the signature
		sig := <-sigChan

		// stop the measurement
		rPrepare.Measure()
		// verifies it
		if err := verifyBlockSignature(node.Suite(), node.EntityList().Aggregate, &sig); err != nil {
			dbg.Lvl1("Round", round, " FAILED")
		} else {
			dbg.Lvl1("Round", round, " SUCCESS")
		}
	}
	return nil
}

func verifyBlockSignature(suite abstract.Suite, aggregate abstract.Point, sig *BlockSignature) error {
	marshalled, err := sig.Block.MarshalBinary()
	if err != nil {
		return fmt.Errorf("Marshalling of block did not work: %v", err)
	}
	return cosi.VerifySignatureWithException(suite, aggregate, marshalled, sig.Sig.Challenge, sig.Sig.Response, sig.Exceptions)
}

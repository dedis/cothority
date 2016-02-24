package byzcoin

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
	sda.SimulationRegister("ByzCoinNaiveSimulation", NewNaiveSimulation)
	sda.ProtocolRegisterName("NaiveByzCoin", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewNtreeProtocol(n) })
}

// Simulation implements da.Simulation interface
type NaiveSimulation struct {
	// sda fields:
	sda.SimulationBFTree
	// your simulation specific fields:
	simulationConfig
}

type naiveSimulationConfig struct {
	// Blocksize is the number of transactions in one block:
	Blocksize int
	// number of transactions the client will send:
	NumClientTxs int
	//blocksDir is the directory where to find the transaction blocks (.dat files)
	BlocksDir string
	// timeout the leader after TimeoutMs milliseconds
	TimeoutMs uint64
	// Fail:
	// 0  do not fail
	// 1 fail by doing nothing
	// 2 fail by sending wrong blocks
	Fail uint8
}

func NewNaiveSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation interface
func (e *NaiveSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run implements sda.Simulation interface
func (e *NaiveSimulation) Run(sdaConf *sda.SimulationConfig) error {
	dbg.Lvl1("Simulation starting with:  Rounds=", e.Rounds)
	server := NewNaiveServer(e.Blocksize)
	client := NewClient(server)
	go client.StartClientSimulation(e.BlocksDir, e.NumClientTxs)
	sigChan := server.BlockSignaturesChan()
	/*var rChallComm *monitor.Measure*/
	/*var rRespPrep *monitor.Measure*/
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		// create an empty node
		node, err := sdaConf.Overlay.NewNodeEmptyName("ByzCoin", sdaConf.Tree)
		if err != nil {
			return err
		}
		// instantiate a byzcoin protocol
		rComplete := monitor.NewMeasure("round_prepare")
		//pi, err := server.Instantiate(node, e.TimeoutMs /*, e.Fail*/)
		_, err = server.Instantiate(node, e.TimeoutMs /*, e.Fail*/)
		if err != nil {
			return err
		}

		// wait for the signature (all steps finished)
		dbg.Print("after instantiate")
		// wait for the signature
		sig := <-sigChan

		rComplete.Measure()
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

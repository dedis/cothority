package pbft

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
)

var magicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

func init() {
	sda.SimulationRegister("PbftSimulation", NewSimulation)
	sda.ProtocolRegisterName("PBFT", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewProtocol(n) })
}

// Simulation implements sda.Simulation interface
type Simulation struct {
	// sda fields:
	sda.SimulationBFTree
	// pbft simulation specific fields:
	// TODO
	// Blocksize is the number of transactions in one block:
	Blocksize int
	//blocksDir is the directory where to find the transaction blocks (.dat files)
	BlocksDir string
}

func NewSimulation(config string) (sda.Simulation, error) {
	sim := &Simulation{}
	_, err := toml.Decode(config, sim)
	if err != nil {
		return nil, err
	}
	return sim, nil
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

func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	// TODO
	doneChan := make(chan bool)
	doneCB := func() { doneChan <- true }
	// FIXME use client instead
	parser, err := blockchain.NewParser(e.BlocksDir, magicNum)
	if err != nil {
		dbg.Error("Couldn't parse blocks in", e.BlocksDir)
		return err
	}
	transactions := parser.Parse(0, e.Blocksize)
	node, err := sdaConf.Overlay.CreateNewNodeName("PBFT", sdaConf.Tree)
	if err != nil {
		return err
	}
	// FIXME c&p from bizcoin.go
	trlist := blockchain.NewTransactionList(transactions, len(transactions))
	header := blockchain.NewHeader(trlist, "", "")
	trblock := blockchain.NewTrBlock(trlist, header)

	proto, err := NewRootProtocol(node, trblock, doneCB)
	if err != nil {
		return err
	}

	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		r := monitor.NewMeasure("round_pbft")
		err := proto.PrePrepare()
		if err != nil {
			dbg.Error("Couldn't start PrePrepare")
			return err
		}

		// wait for finishing pbft:
		<-doneChan
		r.Measure()
		dbg.Lvl1("Finished round", round)
	}
	return nil
}

// helper functions:
// FIXME refactor, use the same mechanisms as in buîzcoin.go's simulation

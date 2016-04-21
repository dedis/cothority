package pbft

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
	"github.com/dedis/cothority/protocols/manage"
)

var magicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

func init() {
	sda.SimulationRegister("ByzCoinPBFT", NewSimulation)
	sda.ProtocolRegisterName("ByzCoinPBFT", func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) { return NewProtocol(n) })
}

// Simulation implements sda.Simulation interface
type Simulation struct {
	// sda fields:
	sda.SimulationBFTree
	// pbft simulation specific fields:
	// Blocksize is the number of transactions in one block:
	Blocksize int
}

// NewSimulation returns a pbft simulation
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
	err := blockchain.EnsureBlockIsAvailable(dir)
	if err != nil {
		dbg.Fatal("Couldn't get block:", err)
	}

	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err = e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run runs the simulation
func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	doneChan := make(chan bool)
	doneCB := func() {
		doneChan <- true
	}
	// FIXME use client instead
	dir := blockchain.GetBlockDir()
	parser, err := blockchain.NewParser(dir, magicNum)
	if err != nil {
		dbg.Error("Error: Couldn't parse blocks in", dir)
		return err
	}
	transactions, err := parser.Parse(0, e.Blocksize)
	if err != nil {
		dbg.Error("Error while parsing transactions", err)
		return err
	}

	// FIXME c&p from byzcoin.go
	trlist := blockchain.NewTransactionList(transactions, len(transactions))
	header := blockchain.NewHeader(trlist, "", "")
	trblock := blockchain.NewTrBlock(trlist, header)

	// Here we first setup the N^2 connections with a broadcast protocol
	pi, err := sdaConf.Overlay.CreateProtocol(sdaConf.Tree, "Broadcast")
	if err != nil {
		dbg.Error(err)
	}
	proto := pi.(*manage.Broadcast)
	// channel to notify we are done
	broadDone := make(chan bool)
	proto.RegisterOnDone(func() {
		broadDone <- true
	})

	// ignore error on purpose: Start always returns nil
	_ = proto.Start()

	// wait
	<-broadDone
	dbg.Lvl3("Simulation can start!")
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		p, err := sdaConf.Overlay.CreateProtocol(sdaConf.Tree, "ByzCoinPBFT")
		if err != nil {
			return err
		}
		proto := p.(*Protocol)

		proto.trBlock = trblock
		proto.onDoneCB = doneCB

		r := monitor.NewTimeMeasure("round_pbft")
		err = proto.Start()
		if err != nil {
			dbg.Error("Couldn't start PrePrepare")
			return err
		}

		// wait for finishing pbft:
		<-doneChan
		r.Record()

		dbg.Lvl2("Finished round", round)
	}
	return nil
}

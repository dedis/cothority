package main

import (
	"github.com/BurntSushi/toml"
	"gopkg.in/dedis/cothority.v1/byzcoin/blockchain"
	"gopkg.in/dedis/cothority.v1/byzcoin/pbft"
	"gopkg.in/dedis/cothority.v1/messaging"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/simul/monitor"
)

var magicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

func init() {
	onet.SimulationRegister("ByzCoinPBFT", NewPBFTSimulation)
	onet.GlobalProtocolRegister("ByzCoinPBFT", func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return pbft.NewProtocol(n)
	})
}

// PBFTSimulation implements onet.Simulation interface
type PBFTSimulation struct {
	// onet fields:
	onet.SimulationBFTree
	// pbft simulation specific fields:
	// Blocksize is the number of transactions in one block:
	Blocksize int
}

// NewPBFTSimulation returns a pbft simulation
func NewPBFTSimulation(config string) (onet.Simulation, error) {
	sim := &PBFTSimulation{}
	_, err := toml.Decode(config, sim)
	if err != nil {
		return nil, err
	}
	return sim, nil
}

// Setup implements onet.Simulation interface
func (e *PBFTSimulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
	err := blockchain.EnsureBlockIsAvailable(dir)
	if err != nil {
		log.Fatal("Couldn't get block:", err)
	}

	sc := &onet.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err = e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run runs the simulation
func (e *PBFTSimulation) Run(onetConf *onet.SimulationConfig) error {
	doneChan := make(chan bool)
	doneCB := func() {
		doneChan <- true
	}
	// FIXME use client instead
	dir := blockchain.GetBlockDir()
	parser, err := blockchain.NewParser(dir, magicNum)
	if err != nil {
		log.Error("Error: Couldn't parse blocks in", dir)
		return err
	}
	transactions, err := parser.Parse(0, e.Blocksize)
	if err != nil {
		log.Error("Error while parsing transactions", err)
		return err
	}

	// FIXME c&p from byzcoin.go
	trlist := blockchain.NewTransactionList(transactions, len(transactions))
	header := blockchain.NewHeader(trlist, "", "")
	trblock := blockchain.NewTrBlock(trlist, header)

	// Here we first setup the N^2 connections with a broadcast protocol
	pi, err := onetConf.Overlay.CreateProtocol("Broadcast", onetConf.Tree, onet.NilServiceID)
	if err != nil {
		log.Error(err)
	}
	proto := pi.(*messaging.Broadcast)
	// channel to notify we are done
	broadDone := make(chan bool)
	proto.RegisterOnDone(func() {
		broadDone <- true
	})

	// ignore error on purpose: Start always returns nil
	_ = proto.Start()

	// wait
	<-broadDone
	log.Lvl3("Simulation can start!")
	for round := 0; round < e.Rounds; round++ {
		log.Lvl1("Starting round", round)
		p, err := onetConf.Overlay.CreateProtocol("ByzCoinPBFT", onetConf.Tree, onet.NilServiceID)
		if err != nil {
			return err
		}
		proto := p.(*pbft.Protocol)

		proto.TrBlock = trblock
		proto.OnDoneCB = doneCB

		r := monitor.NewTimeMeasure("round_pbft")
		err = proto.Start()
		if err != nil {
			log.Error("Couldn't start PrePrepare")
			return err
		}

		// wait for finishing pbft:
		<-doneChan
		r.Record()

		log.Lvl2("Finished round", round)
	}
	return nil
}

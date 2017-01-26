package main

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/blockchain"
	"github.com/dedis/cothority/byzcoin/ntree"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/simul/monitor"
)

func init() {
	onet.SimulationRegister("ByzCoinNtree", NewNTreeSimulation)
	onet.GlobalProtocolRegister("ByzCoinNtree", func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return byzcoinNtree.NewNtreeProtocol(n)
	})
}

// NTreeSimulation implements da.Simulation interface
type NTreeSimulation struct {
	// onet fields:
	onet.SimulationBFTree
	// your simulation specific fields:
	ByzCoinSimulationConfig
}

// NewNTreeSimulation returns a new Ntree simulation
func NewNTreeSimulation(config string) (onet.Simulation, error) {
	es := &NTreeSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation interface
func (e *NTreeSimulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
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

// Run implements onet.Simulation interface
func (e *NTreeSimulation) Run(onetConf *onet.SimulationConfig) error {
	log.Lvl2("Naive Tree Simulation starting with: Rounds=", e.Rounds)
	server := byzcoinNtree.NewNtreeServer(e.Blocksize)
	for round := 0; round < e.Rounds; round++ {
		client := byzcoin.NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), e.Blocksize)
		if err != nil {
			log.Error("ClientSimulation:", err)
		}

		log.Lvl1("Starting round", round)
		// create an empty node
		node := onetConf.Overlay.NewTreeNodeInstanceFromProtoName(onetConf.Tree, "ByzCoinNtree")
		// instantiate a byzcoin protocol
		rComplete := monitor.NewTimeMeasure("round")
		pi, err := server.Instantiate(node)
		if err != nil {
			return err
		}
		onetConf.Overlay.RegisterProtocolInstance(pi)

		nt := pi.(*byzcoinNtree.Ntree)
		// Register when the protocol is finished (all the nodes have finished)
		done := make(chan bool)
		nt.RegisterOnDone(func(sig *byzcoinNtree.NtreeSignature) {
			rComplete.Record()
			log.Lvl3("Done")
			done <- true
		})

		go func() {
			if err := nt.Start(); err != nil {
				log.Error("Couldn't start ntree protocol:", err)
			}
		}()
		// wait for the end
		<-done
		log.Lvl3("Round", round, "finished")

	}
	return nil
}

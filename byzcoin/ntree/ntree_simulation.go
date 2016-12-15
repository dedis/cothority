package byzcoinNtree

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/blockchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/simul/monitor"
)

func init() {
	onet.SimulationRegister("ByzCoinNtree", NewSimulation)
	onet.GlobalProtocolRegister("ByzCoinNtree", func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) { return NewNtreeProtocol(n) })
}

// Simulation implements da.Simulation interface
type Simulation struct {
	// sda fields:
	onet.SimulationBFTree
	// your simulation specific fields:
	byzcoin.SimulationConfig
}

// NewSimulation returns a new Ntree simulation
func NewSimulation(config string) (onet.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation interface
func (e *Simulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
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
func (e *Simulation) Run(sdaConf *onet.SimulationConfig) error {
	log.Lvl2("Naive Tree Simulation starting with: Rounds=", e.Rounds)
	server := NewNtreeServer(e.Blocksize)
	for round := 0; round < e.Rounds; round++ {
		client := byzcoin.NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), e.Blocksize)
		if err != nil {
			log.Error("ClientSimulation:", err)
		}

		log.Lvl1("Starting round", round)
		// create an empty node
		node := sdaConf.Overlay.NewTreeNodeInstanceFromProtoName(sdaConf.Tree, "ByzCoinNtree")
		// instantiate a byzcoin protocol
		rComplete := monitor.NewTimeMeasure("round")
		pi, err := server.Instantiate(node)
		if err != nil {
			return err
		}
		sdaConf.Overlay.RegisterProtocolInstance(pi)

		nt := pi.(*Ntree)
		// Register when the protocol is finished (all the nodes have finished)
		done := make(chan bool)
		nt.RegisterOnDone(func(sig *NtreeSignature) {
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

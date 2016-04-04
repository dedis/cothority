package byzcoinNtree

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
)

func init() {
	sda.SimulationRegister("ByzCoinNtree", NewSimulation)
	sda.RegisterNewProtocol("Ntree", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewNtreeProtocol(n) })
}

// Simulation implements da.Simulation interface
type Simulation struct {
	// sda fields:
	sda.SimulationBFTree
	// your simulation specific fields:
	byzcoin.SimulationConfig
}

// NewSimulation returns a new Ntree simulation
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

// Run implements sda.Simulation interface
func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	dbg.Lvl2("Naive Tree Simulation starting with: Rounds=", e.Rounds)
	server := NewNtreeServer(e.Blocksize)
	for round := 0; round < e.Rounds; round++ {
		client := byzcoin.NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), e.Blocksize)
		if err != nil {
			dbg.Error("ClientSimulation:", err)
		}

		dbg.Lvl1("Starting round", round)
		// create an empty node
<<<<<<< HEAD:protocols/byzcoin/ntree_simulation.go
		node, err := sdaConf.Overlay.NewNodeEmpty("Ntree", sdaConf.Tree)
=======
		node, err := sdaConf.Overlay.NewNodeEmptyName("ByzCoinNtree", sdaConf.Tree)
>>>>>>> development:protocols/byzcoin/ntree/ntree_simulation.go
		if err != nil {
			return err
		}
		// instantiate a byzcoin protocol
		rComplete := monitor.NewTimeMeasure("round")
		pi, err := server.Instantiate(node)
		if err != nil {
			return err
		}

		nt := pi.(*Ntree)
		// Register when the protocol is finished (all the nodes have finished)
		done := make(chan bool)
		nt.RegisterOnDone(func(sig *NtreeSignature) {
			rComplete.Record()
			dbg.Lvl3("Done")
			done <- true
		})

		go nt.Start()
		// wait for the end
		<-done
		dbg.Lvl3("Round", round, "finished")

	}
	return nil
}

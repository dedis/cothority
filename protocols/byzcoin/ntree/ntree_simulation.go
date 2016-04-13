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
	sda.ProtocolRegisterName("ByzCoinNtree", func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) { return NewNtreeProtocol(n) })
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
			dbg.Lvl3("Done")
			done <- true
		})

		go func() {
			if err := nt.Start(); err != nil {
				dbg.Error("Couldn't start ntree protocol:", err)
			}
		}()
		// wait for the end
		<-done
		dbg.Lvl3("Round", round, "finished")

	}
	return nil
}

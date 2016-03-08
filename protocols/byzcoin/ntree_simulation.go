package byzcoin

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
)

func init() {
	sda.SimulationRegister("SimulationNtree", NewSimulationNtree)
	sda.ProtocolRegisterName("Ntree", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewNtreeProtocol(n) })
}

// Simulation implements da.Simulation interface
type SimulationNtree struct {
	// sda fields:
	sda.SimulationBFTree
	// your simulation specific fields:
	SimulationByzCoinConfig
}

func NewSimulationNtree(config string) (sda.Simulation, error) {
	es := &SimulationNtree{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation interface
func (e *SimulationNtree) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
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
func (e *SimulationNtree) Run(sdaConf *sda.SimulationConfig) error {
	dbg.Lvl1("Naive Tree Simulation starting with:  Rounds=", e.Rounds)
	server := NewNtreeServer(e.Blocksize)
	/*var rChallComm monitorMut*/
	/*var rRespPrep monitorMut*/
	for round := 0; round < e.Rounds; round++ {
		client := NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), e.Blocksize)
		if err != nil {
			dbg.Error("ClientSimulation:", err)
		}

		dbg.Lvl1("Starting round", round)
		// create an empty node
		node, err := sdaConf.Overlay.NewNodeEmptyName("Ntree", sdaConf.Tree)
		if err != nil {
			return err
		}
		// instantiate a byzcoin protocol
		rComplete := monitor.NewMeasure("round")
		pi, err := server.Instantiate(node)
		if err != nil {
			return err
		}

		nt := pi.(*Ntree)
		// Register when the protocol is finished (all the nodes have finished)
		done := make(chan bool)
		nt.RegisterOnDone(func(sig *NtreeSignature) {
			rComplete.Measure()
			dbg.Lvl3("NtreeProtocol DONE")
			done <- true
			// TODO verification of signatures
			/*for {*/
			//if err := verifyBlockSignature(node.Suite(), node.EntityList().Aggregate, sig); err != nil {
			//dbg.Lvl1("Round", round, " FAILED:", err)
			//} else {
			//dbg.Lvl1("Round", round, " SUCCESS")
			//}
			/*}*/
		})

		go nt.Start()
		// wait for the end
		<-done
		dbg.Lvl3("Round", round, "finished")

	}
	return nil
}

package cosi

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.SimulationRegister("CoSi", NewSimulation)
}

// Simulation implements the sda.Simulation of the CoSi protocol.
type Simulation struct {
	sda.SimulationBFTree

	// 0 - don't check any signatures
	// 1 - only the root-node checks the aggregate signature
	// 2 - every node checks the aggregate signature
	Checking int
}

// NewSimulation returns an sda.Simulation or an error if sth. is wrong.
// Used to register the CoSi protocol.
func NewSimulation(config string) (sda.Simulation, error) {
	cs := &Simulation{Checking: 2}
	_, err := toml.Decode(config, cs)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

// Setup implements sda.Simulation.
func (cs *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	cs.CreateEntityList(sim, hosts, 2000)
	err := cs.CreateTree(sim)
	return sim, err
}

// Node implements sda.Simulation.
func (cs *Simulation) Node(sc *sda.SimulationConfig) error {
	err := cs.SimulationBFTree.Node(sc)
	if err != nil {
		return err
	}
	VerifyResponse = cs.Checking
	return nil
}

// Run implements sda.Simulation.
func (cs *Simulation) Run(config *sda.SimulationConfig) error {
	size := len(config.EntityList.List)
	msg := []byte("Hello World Cosi Simulation")
	aggPublic := computeAggregatedPublic(config.EntityList)
	dbg.Lvl2("Simulation starting with: Size=", size, ", Rounds=", cs.Rounds)
	for round := 0; round < cs.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		roundM := monitor.NewTimeMeasure("round")
		// create the node with the protocol, but do NOT start it yet.
		node, err := config.Overlay.CreateProtocol(config.Tree, "CoSi")
		if err != nil {
			return err
		}
		// the protocol itself
		proto := node.(*ProtocolCosi)
		// give the message to sign
		proto.SigningMessage(msg)
		// tell us when it is done
		done := make(chan bool)
		fn := func(chal, resp abstract.Secret) {
			roundM.Record()
			//  No need to verify it twice here. It already happens in
			//  handleResponse() even for the root.
			if err := cosi.VerifySignature(network.Suite, msg, aggPublic, chal, resp); err != nil {
				dbg.Lvl1("Round", round, " => fail verification")
			} else {
				dbg.Lvl2("Round", round, " => success")
			}
			done <- true
		}
		proto.RegisterDoneCallback(fn)
		if err := proto.Start(); err != nil {
			dbg.Error("Couldn't start protocol in round", round)
		}
		<-done
	}
	dbg.Lvl1("Simulation finished")
	return nil
}

func computeAggregatedPublic(el *sda.EntityList) abstract.Point {
	suite := network.Suite
	agg := suite.Point().Null()
	for _, e := range el.List {
		agg = agg.Add(agg, e.Public)
	}
	return agg
}

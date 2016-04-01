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
	sda.SimulationRegister("CoSi", NewCoSiSimulation)
	// default protocol initialization. See Run() for override this one for the
	// root.
	sda.RegisterNewProtocol("ProtocolCosi", func(node *sda.Node) (sda.ProtocolInstance, error) { return NewProtocolCosi(node) })
}

type CoSiSimulation struct {
	sda.SimulationBFTree

	// Do we want to check signature at each level, only the root or nothing at
	// all ?
	// See https://github.com/dedis/cothority/issues/260
	Checking int
}

func NewCoSiSimulation(config string) (sda.Simulation, error) {
	cs := new(CoSiSimulation)
	cs.Checking = 2
	_, err := toml.Decode(config, cs)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (cs *CoSiSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	cs.CreateEntityList(sim, hosts, 2000)
	err := cs.CreateTree(sim)
	return sim, err
}

func (cs *CoSiSimulation) Node(sc *sda.SimulationConfig) error {
	err := cs.SimulationBFTree.Node(sc)
	if err != nil {
		return err
	}
	VerifyResponse = cs.Checking
	return nil
}

func (cs *CoSiSimulation) Run(config *sda.SimulationConfig) error {
	size := len(config.EntityList.List)
	msg := []byte("Hello World Cosi Simulation")
	aggPublic := computeAggregatedPublic(config.EntityList)
	dbg.Lvl1("Simulation starting with: Size=", size, ", Rounds=", cs.Rounds)
	for round := 0; round < cs.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		roundM := monitor.NewTimeMeasure("round")
		// create the node with the protocol, but do NOT start it yet.
		node, err := config.Overlay.CreateNewNode("ProtocolCosi", config.Tree)
		if err != nil {
			return err
		}
		// the protocol itself
		proto := node.ProtocolInstance().(*ProtocolCosi)
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
				dbg.Lvl1("Round", round, " => success")
			}
			done <- true
		}
		proto.RegisterDoneCallback(fn)
		proto.Start()
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

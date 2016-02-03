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
	sda.SimulationRegister("CoSiSimulation", NewCoSiSimulation)
	// default protocol initialization. See Run() for override this one for the
	// root.
	sda.ProtocolRegisterName("ProtocolCosi", func(node *sda.Node) (sda.ProtocolInstance, error) { return NewProtocolCosi(node) })
}

type CoSiSimulation struct {
	sda.SimulationBFTree
}

func NewCoSiSimulation(config string) (sda.Simulation, error) {
	cs := new(CoSiSimulation)
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

func (cs *CoSiSimulation) Run(config *sda.SimulationConfig) error {
	size := len(config.EntityList.List)
	msg := []byte("Hello World Cosi Simulation")
	aggPublic := computeAggregatedPublic(config.EntityList)
	dbg.Lvl1("Simulation starting with: SIZE=", size, ", ROUNDS=", cs.Rounds)
	for round := 0; round < cs.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		round := monitor.NewMeasure("round")
		// create the node with the protocol, but do NOT start it yet.
		node, err := config.Overlay.CreateNewNodeName("ProtocolCosi", config.Tree)
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
			round.Measure()
			if err := cosi.VerifySignature(network.Suite, msg, aggPublic, chal, resp); err != nil {
				dbg.Lvl1("Round verification FAIL !!")
			} else {
				dbg.Lvl1("Round finished SUCESS")
			}
			done <- true
			// TODO make the verification here
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

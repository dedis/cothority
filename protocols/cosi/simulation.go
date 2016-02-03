package cosi

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
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
	size := config.Tree.Size()
	msg := []byte("Hello World Cosi Simulation")
	dbg.Lvl2("Size is:", size, "rounds:", cs.Rounds)
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
			done <- true
			// TODO make the verification here
		}
		proto.RegisterDoneCallback(fn)
		proto.Start()
		<-done
	}
	return nil
}

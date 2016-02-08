package jvss

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	// FIXME Protocol doesn't exists:
	sda.SimulationRegister("SimulationJVSS", NewJvssSimulation)
	sda.ProtocolRegisterName("ProtocolCosi", func(node *sda.Node) (sda.ProtocolInstance, error) { return NewJVSSProtocolInstance(node) })
}

type JvssSimulation struct {
	sda.SimulationBFTree
}

func NewJvssSimulation(config string) (sda.Simulation, error) {
	es := &JvssSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

func (jv *JvssSimulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	jv.CreateEntityList(sc, hosts, 2000)
	err := jv.CreateTree(sc)
	return sc, err
}

func (jv *JvssSimulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	dbg.Lvl2("Size is:", size, "rounds:", jv.Rounds)
	// compute long-term secret:
	// node, err := config.Overlay.CreateNewNodeName("ProtocolCosi", config.Tree)
	for round := 0; round < jv.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		roundMeasure := monitor.NewMeasure("round")
		_, err := config.Overlay.StartNewNodeName("Jvss", config.Tree)
		if err != nil {
			return err
		}
		roundMeasure.Measure()
	}
	return nil
}

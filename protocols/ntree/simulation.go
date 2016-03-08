package ntree

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("NaiveTreeSimulation", NewSimulation)
	sda.ProtocolRegisterName("CoSiNtree", func(node *sda.Node) (sda.ProtocolInstance, error) { return NewProtocol(node) })
}

type Simulation struct {
	sda.SimulationBFTree
	Message string
}

func NewSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

func (e *Simulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (e *Simulation) Run(config *sda.SimulationConfig) error {
	msg := []byte(e.Message)
	size := config.Tree.Size()
	dbg.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round, "with message", msg)
		round := monitor.NewMeasure("round")

		node, err := config.Overlay.NewNodeEmptyName("CoSiNtree", config.Tree)
		if err != nil {
			dbg.Error("Quitting the simuation....", err)
			return err
		}
		proto, err := NewRootProtocol(msg, node)
		if err != nil {
			dbg.Error("Quitting the simuation....", err)
			return err
		}
		node.SetProtocolInstance(proto)
		err = proto.Start()
		if err != nil {
			dbg.Error("Quitting the simuation....", err)
			return err
		}
		round.Measure()
	}
	return nil
}

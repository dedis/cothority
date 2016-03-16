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
	Message  string
	Checking int
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
		dbg.Lvl1("Starting round", round, "with message", string(msg))
		round := monitor.NewTimeMeasure("round")

		node, err := config.Overlay.CreateNewNodeName("CoSiNtree", config.Tree)
		if err != nil {
			dbg.Error("Quitting the simulation....", err)
			return err
		}
		pi := node.ProtocolInstance().(*Protocol)
		pi.SetMessage(msg)
		pi.verifySignature = e.Checking

		done := make(chan bool)
		node.OnDoneCallback(func() bool {
			done <- true
			return true
		})
		err = node.Start()
		if err != nil {
			dbg.Error("Quitting the simulation....", err)
			return err
		}
		<-done
		round.Record()
	}
	return nil
}

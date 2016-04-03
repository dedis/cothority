package jvss

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("JVSS", NewSimulation)
}

// Simulation implements a JVSS simulation
type Simulation struct {
	sda.SimulationBFTree
	Verify bool
}

// NewSimulation creates a JVSS simulation
func NewSimulation(config string) (sda.Simulation, error) {
	jvs := &Simulation{Verify: true}
	_, err := toml.Decode(config, jvs)
	if err != nil {
		return nil, err
	}
	return jvs, nil
}

// Setup configures a JVSS simulation
func (jvs *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	jvs.CreateEntityList(sim, hosts, 2000)
	err := jvs.CreateTree(sim)
	return sim, err
}

// Run initiates a JVSS simulation
func (jvs *Simulation) Run(config *sda.SimulationConfig) error {

	size := config.Tree.Size()
	msg := []byte("Test message for JVSS simulation")

	dbg.Lvl1("Size:", size, "rounds:", jvs.Rounds)

	node, err := config.Overlay.CreateNewNodeName("JVSS", config.Tree)
	if err != nil {
		return err
	}
	proto := node.ProtocolInstance().(*JVSS)

	dbg.Lvl1("JVSS - starting")
	node.StartProtocol()
	dbg.Lvl1("JVSS - setup done")

	for round := 0; round < jvs.Rounds; round++ {
		dbg.Lvl1("JVSS - starting round", round)
		r := monitor.NewTimeMeasure("round")
		dbg.Lvl1("JVSS - requesting signature")
		sig, err := proto.Sign(msg)
		if err != nil {
			dbg.Error("JVSS - could not create signature")
			return err
		}
		if jvs.Verify {
			dbg.Lvl1("JVSS - signature received")
			if err := proto.Verify(msg, sig); err != nil {
				dbg.Error("JVSS - invalid signature")
				return err
			}
			dbg.Lvl1("JVSS - signature verification succeded")
		}
		r.Record()
	}

	return nil
}

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

	p, err := config.Overlay.CreateProtocol(config.Tree, "JVSS")
	if err != nil {
		return err
	}
	proto := p.(*JVSS)

	dbg.Lvl1("Starting setup")
	proto.Start()
	dbg.Lvl1("Setup done")

	for round := 0; round < jvs.Rounds; round++ {
		dbg.Lvl1("Starting signing round", round)
		r := monitor.NewTimeMeasure("round")
		dbg.Lvl2("Requesting signature")
		sig, err := proto.Sign(msg)
		if err != nil {
			dbg.Error("Could not create signature")
			return err
		}
		if jvs.Verify {
			dbg.Lvl2("Signature received")
			if err := proto.Verify(msg, sig); err != nil {
				dbg.Error("Signature invalid")
				return err
			}
			dbg.Lvl2("Signature valid")
		}
		r.Record()
	}

	return nil
}

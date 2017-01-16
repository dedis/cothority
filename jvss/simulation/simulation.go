package main

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/jvss"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/simul/monitor"
)

func init() {
	onet.SimulationRegister("JVSS", NewSimulation)
}

// Simulation implements a JVSS simulation
type Simulation struct {
	onet.SimulationBFTree
	Verify bool
}

// NewSimulation creates a JVSS simulation
func NewSimulation(config string) (onet.Simulation, error) {
	jvs := &Simulation{Verify: true}
	_, err := toml.Decode(config, jvs)
	if err != nil {
		return nil, err
	}
	return jvs, nil
}

// Setup configures a JVSS simulation
func (jvs *Simulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
	sim := new(onet.SimulationConfig)
	jvs.CreateRoster(sim, hosts, 2000)
	err := jvs.CreateTree(sim)
	return sim, err
}

// Run initiates a JVSS simulation
func (jvs *Simulation) Run(config *onet.SimulationConfig) error {

	size := config.Tree.Size()
	msg := []byte("Test message for JVSS simulation")

	log.Lvl1("Size:", size, "rounds:", jvs.Rounds)

	p, err := config.Overlay.CreateProtocol("JVSS", config.Tree, onet.NilServiceID)
	if err != nil {
		return err
	}
	proto := p.(*jvss.JVSS)

	log.Lvl1("Starting setup")
	proto.Start()
	log.Lvl1("Setup done")

	for round := 0; round < jvs.Rounds; round++ {
		log.Lvl1("Starting signing round", round)
		r := monitor.NewTimeMeasure("round")
		log.Lvl2("Requesting signature")
		sig, err := proto.Sign(msg)
		if err != nil {
			log.Error("Could not create signature")
			return err
		}
		if jvs.Verify {
			log.Lvl2("Signature received")
			if err := proto.Verify(msg, sig); err != nil {
				log.Error("Signature invalid")
				return err
			}
			log.Lvl2("Signature valid")
		}
		r.Record()
	}

	return nil
}

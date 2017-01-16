package main

import (
	"errors"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/example/channels"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/simul/monitor"
)

/*
This is a simple ExampleChannels-protocol with two steps:
- announcement - which sends a message to all children
- reply - used for counting the number of children
*/

func init() {
	onet.SimulationRegister("ExampleChannels", NewChannelSimulation)
}

// ChannelSimulation implements onet.Simulation.
type ChannelSimulation struct {
	onet.SimulationBFTree
}

// NewChannelSimulation is used internally to register the simulation.
func NewChannelSimulation(config string) (onet.Simulation, error) {
	es := &ChannelSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation.
func (e *ChannelSimulation) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run implements onet.Simulation.
func (e *ChannelSimulation) Run(config *onet.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		log.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		p, err := config.Overlay.CreateProtocol("ExampleChannels", config.Tree, onet.NilServiceID)
		if err != nil {
			return err
		}
		go p.Start()
		children := <-p.(*channels.ProtocolExampleChannels).ChildCount
		round.Record()
		if children != size {
			return errors.New("Didn't get " + strconv.Itoa(size) +
				" children")
		}
	}
	return nil
}

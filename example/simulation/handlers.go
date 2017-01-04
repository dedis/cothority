package main

import (
	"errors"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/example/handlers"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/simul/monitor"
)

/*
This is a simple ExampleHandlers-protocol with two steps:
- announcement - which sends a message to all children
- reply - used for counting the number of children
*/

func init() {
	onet.SimulationRegister("ExampleHandlers", NewHandlerSimulation)
}

// HandlerSimulation implements onet.Simulation.
type HandlerSimulation struct {
	onet.SimulationBFTree
}

// NewHandlerSimulation is used internally to register the simulation (see the init()
// function above).
func NewHandlerSimulation(config string) (onet.Simulation, error) {
	es := &HandlerSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation.
func (e *HandlerSimulation) Setup(dir string, hosts []string) (
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
func (e *HandlerSimulation) Run(config *onet.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		log.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		p, err := config.Overlay.CreateProtocolOnet("ExampleHandlers", config.Tree)
		if err != nil {
			return err
		}
		go p.Start()
		children := <-p.(*handlers.ProtocolExampleHandlers).ChildCount
		round.Record()
		if children != size {
			return errors.New("Didn't get " + strconv.Itoa(size) +
				" children")
		}
	}
	return nil
}

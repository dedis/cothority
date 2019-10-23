package main

import (
	"github.com/BurntSushi/toml"
	"go.dedis.ch/cothority/v4/cosi/crypto"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/simul/monitor"
)

func init() {
	onet.SimulationRegister(Name, NewSimulation)
}

// Simulation implements the onet.Simulation of the CoSi protocol.
type Simulation struct {
	onet.SimulationBFTree
	Checking VRType
}

// NewSimulation returns an onet.Simulation or an error if sth. is wrong.
// Used to register the CoSi protocol.
func NewSimulation(config string) (onet.Simulation, error) {
	cs := &Simulation{Checking: 2}
	_, err := toml.Decode(config, cs)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

// Setup implements onet.Simulation.
func (cs *Simulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
	sim := new(onet.SimulationConfig)
	cs.CreateRoster(sim, hosts, 2000)
	err := cs.CreateTree(sim)
	return sim, err
}

// Node implements onet.Simulation.
func (cs *Simulation) Node(sc *onet.SimulationConfig) error {
	err := cs.SimulationBFTree.Node(sc)
	if err != nil {
		return err
	}
	return nil
}

// Run implements onet.Simulation.
func (cs *Simulation) Run(config *onet.SimulationConfig) error {
	size := len(config.Roster.List)
	msg := []byte("Hello World Cosi Simulation")
	log.Lvl2("Simulation starting with: Size=", size, ", Rounds=", cs.Rounds)
	for round := 0; round < cs.Rounds; round++ {
		log.Lvl1("Starting round", round)
		roundM := monitor.NewTimeMeasure("round")
		// create the node with the protocol, but do NOT start it yet.
		node, err := config.Overlay.CreateProtocol(Name, config.Tree, onet.NilServiceID)
		if err != nil {
			return err
		}
		// the protocol itself
		proto := node.(*CoSimul)
		// give the message to sign
		proto.SigningMessage(msg)
		proto.VerifyResponse = cs.Checking
		// tell us when it is done
		done := make(chan bool)
		fn := func(sig []byte) {
			roundM.Record()
			publics := proto.Publics()
			if err := crypto.VerifySignature(proto.Suite(), publics, msg, sig); err != nil {
				log.Lvl1("Round", round, " => fail verification")
			} else {
				log.Lvl2("Round", round, " => success")
			}
			done <- true
		}
		proto.RegisterSignatureHook(fn)
		if err := proto.Start(); err != nil {
			log.Error("Couldn't start protocol in round", round)
		}
		<-done
	}
	log.Lvl1("Simulation finished")
	return nil
}

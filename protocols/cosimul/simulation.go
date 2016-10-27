package cosimul

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/cosi"
)

func init() {
	sda.SimulationRegister(Name, NewSimulation)
}

// Simulation implements the sda.Simulation of the CoSi protocol.
type Simulation struct {
	sda.SimulationBFTree
	Checking VRType
}

// NewSimulation returns an sda.Simulation or an error if sth. is wrong.
// Used to register the CoSi protocol.
func NewSimulation(config string) (sda.Simulation, error) {
	cs := &Simulation{Checking: 2}
	_, err := toml.Decode(config, cs)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

// Setup implements sda.Simulation.
func (cs *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	cs.CreateRoster(sim, hosts, 2000)
	err := cs.CreateTree(sim)
	return sim, err
}

// Node implements sda.Simulation.
func (cs *Simulation) Node(sc *sda.SimulationConfig) error {
	err := cs.SimulationBFTree.Node(sc)
	if err != nil {
		return err
	}
	VerifyResponse = cs.Checking
	return nil
}

// Run implements sda.Simulation.
func (cs *Simulation) Run(config *sda.SimulationConfig) error {
	size := len(config.Roster.List)
	msg := []byte("Hello World Cosi Simulation")
	log.Lvl2("Simulation starting with: Size=", size, ", Rounds=", cs.Rounds)
	for round := 0; round < cs.Rounds; round++ {
		log.Lvl1("Starting round", round)
		roundM := monitor.NewTimeMeasure("round")
		// create the node with the protocol, but do NOT start it yet.
		node, err := config.Overlay.CreateProtocolSDA(Name, config.Tree)
		if err != nil {
			return err
		}
		// the protocol itself
		proto := node.(*CoSimul)
		// give the message to sign
		proto.SigningMessage(msg)
		// tell us when it is done
		done := make(chan bool)
		fn := func(sig []byte) {
			roundM.Record()
			publics := proto.Publics()
			if err := cosi.VerifySignature(network.Suite, publics,
				msg, sig); err != nil {
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

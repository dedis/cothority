package cosi

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"golang.org/x/net/context"
)

func init() {
	sda.SimulationRegister("CosiService", NewSimulation)
}

// Simulation implements the sda.Simulation of the CoSi protocol.
type Simulation struct {
	sda.SimulationBFTree
}

// NewSimulation returns an sda.Simulation or an error if sth. is wrong.
// Used to register the CoSi protocol.
func NewSimulation(config string) (sda.Simulation, error) {
	cs := &Simulation{}
	_, err := toml.Decode(config, cs)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

// Setup implements sda.Simulation.
func (cs *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	cs.CreateEntityList(sim, hosts, 2000)
	err := cs.CreateTree(sim)
	return sim, err
}

// Node implements sda.Simulation.
func (cs *Simulation) Node(sc *sda.SimulationConfig) error {
	err := cs.SimulationBFTree.Node(sc)
	if err != nil {
		return err
	}
	return nil
}

// Run implements sda.Simulation.
func (cs *Simulation) Run(config *sda.SimulationConfig) error {
	size := len(config.EntityList.List)
	msg := []byte("Hello World Cosi Simulation")
	dbg.Lvl2("Simulation starting with: Size=", size, ", Rounds=", cs.Rounds)
	for round := 0; round < cs.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		roundM := monitor.NewTimeMeasure("round")
		// create client
		priv, pub := sda.PrivPub()
		client := network.NewSecureTCPHost(priv, network.NewEntity(pub))

		// connect
		c, err := client.Open(config.Host.Entity)
		if err != nil {
			dbg.Error("Client could not connect to service")
			continue
		}
		// send request
		r := &SignatureRequest{
			Message:    msg,
			EntityList: config.EntityList,
		}
		req, err := sda.CreateServiceRequest(ServiceName, r)
		if err != nil {
			dbg.Error("could not create service request")
			continue
		}
		if err = c.Send(context.TODO(), req); err != nil {
			dbg.Error("Could not send the request")
			continue
		}
		// wait for the response
		nm, err := c.Receive(context.TODO())
		if err != nil {
			dbg.Error("Could not receive the response")
			continue
		}

		resp, ok := nm.Msg.(SignatureResponse)
		if !ok {
			dbg.Error("Received wrong type")
		}

		if err = cosi.VerifySignature(config.Host.Suite(), msg, config.EntityList.Aggregate, resp.Challenge, resp.Response); err != nil {
			dbg.Error("Invalid signature !")
		}
		roundM.Record()
	}
	dbg.Lvl1("Simulation finished")
	return nil
}

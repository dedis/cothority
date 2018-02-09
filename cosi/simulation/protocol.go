package main

/*
The simulation-file can be used with the `cothority/simul` and be run either
locally or on deterlab. Contrary to the `test` of the protocol, the simulation
is much more realistic, as it tests the protocol on different nodes, and not
only in a test-environment.

The Setup-method is run once on the client and will create all structures
and slices necessary to the simulation. It also receives a 'dir' argument
of a directory where it can write files. These files will be copied over to
the simulation so that they are available.

The Run-method is called only once by the root-node of the tree defined in
Setup. It should run the simulation in different rounds. It can also
measure the time each run takes.

In the Node-method you can read the files that have been created by the
'Setup'-method.
*/

import (
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/onet/simul/monitor"
	"github.com/dedis/student_17_bftcosi/protocol"
)

func init() {
	onet.SimulationRegister("CosiProtocol", NewSimulationProtocol)
}

// SimulationProtocol implements onet.Simulation.
type SimulationProtocol struct {
	onet.SimulationBFTree
	NSubtrees         int
	FailingSubleaders int
	FailingLeafs      int
}

// NewSimulationProtocol is used internally to register the simulation (see the init()
// function above).
func NewSimulationProtocol(config string) (onet.Simulation, error) {
	es := &SimulationProtocol{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation.
func (s *SimulationProtocol) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	s.CreateRoster(sc, hosts, 2000)
	err := s.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Node can be used to initialize each node before it will be run
// by the server. Here we call the 'Node'-method of the
// SimulationBFTree structure which will load the roster- and the
// tree-structure to speed up the first round.
func (s *SimulationProtocol) Node(config *onet.SimulationConfig) error {
	index, _ := config.Roster.Search(config.Server.ServerIdentity.ID)
	if index < 0 {
		log.Fatal("Didn't find this node in roster")
	}

	// get subleader ids
	subleadersIds, err := protocol.GetSubleaderIDs(config.Tree, s.Hosts, s.NSubtrees)
	if err != nil {
		return err
	}
	if len(subleadersIds) > s.FailingSubleaders {
		subleadersIds = subleadersIds[:s.FailingSubleaders]
	}

	// get leafs ids
	leafsIds, err := protocol.GetLeafsIDs(config.Tree, s.Hosts, s.NSubtrees)
	if err != nil {
		return err
	}
	if len(leafsIds) > s.FailingLeafs {
		leafsIds = leafsIds[:s.FailingLeafs]
	}

	toIntercept := append(leafsIds, subleadersIds...)

	// intercept announcements on some nodes
	for _, id := range toIntercept {
		if id == config.Server.ServerIdentity.ID {
			config.Server.RegisterProcessorFunc(onet.ProtocolMsgID, func(e *network.Envelope) {
				//get message
				_, msg, err := network.Unmarshal(e.Msg.(*onet.ProtocolMsg).MsgSlice, config.Server.Suite())
				if err != nil {
					log.Fatal("error while unmarshaling a message:", err)
					return
				}

				switch msg.(type) {
				case *protocol.Announcement, *protocol.Commitment, *protocol.Challenge, *protocol.Response:
					log.Lvl3("ignoring cosi message")
				default:
					config.Overlay.Process(e)
				}
			})
			break // this node has been found
		}
	}
	log.Lvl3("Initializing node-index", index)
	return s.SimulationBFTree.Node(config)
}

// Run implements onet.Simulation.
func (s *SimulationProtocol) Run(config *onet.SimulationConfig) error {
	log.SetDebugVisible(2)
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", s.Rounds)
	for round := 0; round < s.Rounds; round++ {
		log.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")

		proposal := []byte{0xFF}
		p, err := config.Overlay.CreateProtocol(protocol.ProtocolName, config.Tree,
			onet.NilServiceID)
		if err != nil {
			return err
		}
		proto := p.(*protocol.CoSiRootNode)
		proto.NSubtrees = s.NSubtrees
		proto.Proposal = proposal
		// timeouts may need to be modified depending on platform
		proto.SubleaderTimeout = protocol.DefaultSubleaderTimeout
		proto.LeavesTimeout = protocol.DefaultLeavesTimeout
		proto.CreateProtocol = func(name string, t *onet.Tree) (onet.ProtocolInstance, error) {
			return config.Overlay.CreateProtocol(name, t, onet.NilServiceID)
		}
		proto.ProtocolTimeout = 10 * time.Second
		go func() {
			log.ErrFatal(p.Start())
		}()
		Signature := <-proto.FinalSignature
		round.Record()

		// get public keys
		publics := make([]kyber.Point, config.Tree.Size())
		for i, node := range config.Tree.List() {
			publics[i] = node.ServerIdentity.Public
		}

		// verify signature
		threshold := s.Hosts - s.FailingLeafs - s.FailingSubleaders
		suite := suites.MustFind(proto.Suite().String())
		err = cosi.Verify(suite, publics, proposal, Signature, cosi.NewThresholdPolicy(threshold))
		if err != nil {
			return fmt.Errorf("error while verifying signature:%s", err)
		}
		log.Lvl2("Signature correctly verified!")

	}
	return nil
}

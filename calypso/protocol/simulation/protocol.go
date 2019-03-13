package main

/**
Simulation for the Calypso-OCS method, to get
*/

import (
	"errors"
	"sync"
	"time"

	"go.dedis.ch/kyber/v3"

	"go.dedis.ch/cothority/v3"

	"go.dedis.ch/onet/v3/network"

	"go.dedis.ch/cothority/v3/calypso/protocol"
	"go.dedis.ch/kyber/v3/share"

	"github.com/BurntSushi/toml"
	dkgprotocol "go.dedis.ch/cothority/v3/dkg/pedersen"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/simul/monitor"
)

var ocsServiceName = "ocsService"

func init() {
	onet.SimulationRegister("OCSSimulation", NewSimulationProtocol)
}

// SimulationProtocol implements onet.Simulation.
type SimulationProtocol struct {
	onet.SimulationBFTree
	Reencryptions int
	Threshold     int
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
func (s *SimulationProtocol) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {
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
	log.Lvl3("Initializing node-index", index)
	return s.SimulationBFTree.Node(config)
}

var propagationTimeout = 10 * time.Second

// Run implements onet.Simulation.
func (s *SimulationProtocol) Run(config *onet.SimulationConfig) error {
	time.Sleep(time.Second)
	threshold := s.Threshold
	serv := config.GetService(ocsServiceName).(*ocsService)
	setup := monitor.NewTimeMeasure("setup")
	err := serv.doDKG(config.Tree, threshold)
	if err != nil {
		return err
	}
	setup.Record()
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", s.Rounds)

	for round := 0; round < s.Rounds; round++ {
		log.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		wg := sync.WaitGroup{}
		wg.Add(s.Reencryptions)
		for i := 0; i < s.Reencryptions; i++ {
			go func(i int) {
				log.Lvl2("Starting", i)
				pi, err := serv.createOCS(config.Tree, threshold)
				log.ErrFatal(err)
				ocs := pi.(*protocol.OCS)
				ocs.Xc = cothority.Suite.Point().Base()
				ocs.U, _ = EncodeKey(ocs.Xc, []byte("secret"))
				err = pi.Start()
				log.ErrFatal(err)
				log.Lvl2("Done", i)
				wg.Done()
			}(i)
		}
		wg.Wait()
		round.Record()
	}
	return nil
}

// ocsService allows setting the dkg-field of the protocol.
type ocsService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	// Has to be initialised by the test
	Shared *dkgprotocol.SharedSecret
	Poly   *share.PubPoly

	done *chan bool
}

// doDKG starts the distributed key generation and each node will store its secret share in its
// service configuration, so that it's read for later createOCS calls.
// This method does only return if there is an error or if the DKG finished successfully.
func (s *ocsService) doDKG(t *onet.Tree, threshold int) error {
	tni := s.NewTreeNodeInstance(t, t.Root, dkgprotocol.Name)
	pi, err := s.NewProtocol(tni, nil)
	if err != nil {
		return err
	}
	d := make(chan bool)
	s.done = &d
	err = s.RegisterProtocolInstance(pi)
	if err != nil {
		return err
	}

	go func() {
		err := pi.Dispatch()
		if err != nil {
			log.Error(err)
		}
	}()

	if err := pi.Start(); err != nil {
		return err
	}

	log.Lvl2("Started DKG-protocol - waiting for done", len(t.List()))
	select {
	case <-d:
		log.Lvlf3("%v got shared %v", s.ServerIdentity(), s.Shared)
		return nil
	case <-time.After(propagationTimeout):
		return errors.New("new-dkg didn't finish in time")
	}
}

// Creates a service-protocol and returns the ProtocolInstance.
func (s *ocsService) createOCS(t *onet.Tree, threshold int) (onet.ProtocolInstance, error) {
	pi, err := s.CreateProtocol(protocol.NameOCS, t)
	if err != nil {
		return nil, err
	}
	ocs := pi.(*protocol.OCS)
	ocs.Shared = s.Shared
	ocs.Poly = s.Poly
	ocs.Threshold = threshold
	return pi, err
}

// Store the dkg in the protocol
func (s *ocsService) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	switch tn.ProtocolName() {
	case dkgprotocol.Name:
		pi, err := dkgprotocol.NewSetup(tn)
		if err != nil {
			return nil, err
		}
		setupDKG := pi.(*dkgprotocol.Setup)
		//setupDKG.KeyPair = key.NewKeyPair(cothority.Suite)
		setupDKG.KeyPair = s.getKeyPair()
		setupDKG.Wait = true

		go func() {
			<-setupDKG.Finished
			shared, dks, err := setupDKG.SharedSecret()
			if err != nil {
				log.Error(err)
				return
			}
			log.Lvlf2("%v got shared %v", s.ServerIdentity(), shared)
			s.Shared = shared
			s.Poly = share.NewPubPoly(cothority.Suite, cothority.Suite.Point().Base(), dks.Commits)
			if s.done != nil {
				*s.done <- true
				s.done = nil
			}
		}()

		return pi, nil
	case protocol.NameOCS:
		pi, err := protocol.NewOCS(tn)
		if err != nil {
			return nil, err
		}
		ocs := pi.(*protocol.OCS)
		ocs.Shared = s.Shared
		return ocs, nil
	default:
		return nil, errors.New("unknown protocol for this service")
	}
}

func (s *ocsService) getKeyPair() *key.Pair {
	tree := onet.NewRoster([]*network.ServerIdentity{s.ServerIdentity()}).GenerateBinaryTree()
	tni := s.NewTreeNodeInstance(tree, tree.Root, "dummy")
	return &key.Pair{
		Public:  tni.Public(),
		Private: tni.Private(),
	}
}

// starts a new service. No function needed.
func newService(c *onet.Context) (onet.Service, error) {
	s := &ocsService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s, nil
}

// EncodeKey can be used by the writer to an onchain-secret skipchain
// to encode his symmetric key under the collective public key created
// by the DKG.
// As this method uses `Pick` to encode the key, depending on the key-length
// more than one point is needed to encode the data.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - key - the symmetric key for the document
//
// Output:
//   - U - the schnorr commit
//   - Cs - encrypted key-slices
func EncodeKey(X kyber.Point, key []byte) (U kyber.Point, Cs []kyber.Point) {
	suite := cothority.Suite
	r := suite.Scalar().Pick(suite.RandomStream())
	C := suite.Point().Mul(r, X)
	log.Lvl3("C:", C.String())
	U = suite.Point().Mul(r, nil)
	log.Lvl3("U is:", U.String())

	for len(key) > 0 {
		var kp kyber.Point
		kp = suite.Point().Embed(key, suite.RandomStream())
		log.Lvl3("Keypoint:", kp.String())
		log.Lvl3("X:", X.String())
		Cs = append(Cs, suite.Point().Add(C, kp))
		log.Lvl3("Cs:", C.String())
		key = key[min(len(key), kp.EmbedLen()):]
	}
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

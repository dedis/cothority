package protocol

import (
	"errors"
	"flag"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

const FailureProtocolName = "FailureProtocol"
const FailureSubProtocolName = "FailureSubProtocol"

func NewFailureProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBlsFtCosi(n, vf, FailureSubProtocolName, testSuite, pairingSuite)
}
func NewFailureSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return false }
	return NewSubBlsFtCosi(n, vf, testSuite, pairingSuite)
}

const RefuseOneProtocolName = "RefuseOneProtocol"
const RefuseOneSubProtocolName = "RefuseOneSubProtocol"

func NewRefuseOneProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBlsFtCosi(n, vf, RefuseOneSubProtocolName, testSuite, pairingSuite)
}
func NewRefuseOneSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return refuse(n, a, b) }
	return NewSubBlsFtCosi(n, vf, testSuite, pairingSuite)
}

// Used for tests
var testServiceID onet.ServiceID

const testServiceName = "ServiceBlsFtCosi"

var newProtocolMethods = map[string]func(*onet.TreeNodeInstance) (onet.ProtocolInstance, error){
	DefaultProtocolName:      NewDefaultProtocol,
	DefaultSubProtocolName:   NewDefaultSubProtocol,
	FailureProtocolName:      NewFailureProtocol,
	FailureSubProtocolName:   NewFailureSubProtocol,
	RefuseOneProtocolName:    NewRefuseOneProtocol,
	RefuseOneSubProtocolName: NewRefuseOneSubProtocol,
}

func init() {
	log.SetDebugVisible(3)
	var err error
	testServiceID, err = onet.RegisterNewService(testServiceName, newService)
	log.ErrFatal(err)

	// Register Protocols
	GlobalRegisterDefaultProtocols()
	onet.GlobalProtocolRegister(FailureProtocolName, NewFailureProtocol)
	onet.GlobalProtocolRegister(FailureSubProtocolName, NewFailureSubProtocol)
	onet.GlobalProtocolRegister(RefuseOneProtocolName, NewRefuseOneProtocol)
	onet.GlobalProtocolRegister(RefuseOneSubProtocolName, NewRefuseOneSubProtocol)
}

var testSuite = cothority.Suite
var pairingSuite = bn256.NewSuite()
var defaultTimeout = 5 * time.Second

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		defaultTimeout = 20 * time.Second
	}
	log.MainTest(m)
}

// Tests various trees configurations
func TestProtocol(t *testing.T) {
	nodes := []int{1, 2, 5, 13, 24}
	subtrees := []int{1, 2, 5, 9}
	proposal := []byte{0xFF}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			if nSubtrees >= nNodes && nSubtrees > 1 {
				continue
			}
			log.Lvl2("test asking for", nNodes, "nodes and", nSubtrees, "subtrees")

			local := onet.NewLocalTest(testSuite)
			servers, _, tree := local.GenTree(nNodes, false)

			services := local.GetServices(servers, testServiceID)
			// Share public keys among services
			sharePublicKeys(services)

			rootService := services[0].(*testService)
			pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}

			cosiProtocol := pi.(*BlsFtCosi)
			cosiProtocol.CreateProtocol = rootService.CreateProtocol
			cosiProtocol.Msg = proposal
			cosiProtocol.NSubtrees = nSubtrees
			cosiProtocol.Timeout = defaultTimeout
			cosiProtocol.Threshold = nNodes

			cosiProtocol.PairingPrivate = rootService.private
			cosiProtocol.PairingPublic = rootService.public
			cosiProtocol.PairingPublics = rootService.pairingPublicKeys

			err = cosiProtocol.Start()
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			}

			// get and verify signature
			_, err = getAndVerifySignature(cosiProtocol, proposal, CompletePolicy{})
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			}

			local.CloseAll()
		}
	}
}

// Tests various trees configurations
func TestProtocolQuickAnswer(t *testing.T) {
	nodes := []int{2, 5, 13, 24}
	subtrees := []int{1, 2, 5, 9}
	proposal := []byte{0xFF}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			if nSubtrees >= nNodes && nSubtrees > 1 {
				continue
			}
			threshold := nNodes / 2
			log.Lvl2("test asking for", nNodes, "node(s),", nSubtrees, "subtree(s) and a", threshold, "node(s) threshold")

			local := onet.NewLocalTest(testSuite)
			servers, _, tree := local.GenTree(nNodes, false)

			services := local.GetServices(servers, testServiceID)
			// Share public keys among services
			sharePublicKeys(services)

			rootService := services[0].(*testService)
			pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}

			cosiProtocol := pi.(*BlsFtCosi)
			cosiProtocol.CreateProtocol = rootService.CreateProtocol
			cosiProtocol.Msg = proposal
			cosiProtocol.NSubtrees = nSubtrees
			cosiProtocol.Timeout = defaultTimeout
			cosiProtocol.Threshold = threshold

			cosiProtocol.PairingPrivate = rootService.private
			cosiProtocol.PairingPublic = rootService.public
			cosiProtocol.PairingPublics = rootService.pairingPublicKeys

			err = cosiProtocol.Start()
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			}

			// get and verify signature
			sig, err := getAndVerifySignature(cosiProtocol, proposal, NewThresholdPolicy(threshold))
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			}

			mask, err := NewMask(cosiProtocol.pairingSuite, cosiProtocol.PairingPublics, -1)
			require.Nil(t, err)
			lenRes := cosiProtocol.pairingSuite.G1().PointLen()
			mask.SetMask(sig[lenRes:])
			// Test that we have less than nNodes signatures
			require.NotEqual(t, nNodes, mask.CountEnabled())

			local.CloseAll()
		}
	}
}

// Tests unresponsive leaves in various tree configurations
func TestUnresponsiveLeafs(t *testing.T) {
	nodes := []int{3, 13, 24}
	subtrees := []int{1, 2}
	proposal := []byte{0xFF}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			log.Lvl2("test asking for", nNodes, "nodes and", nSubtrees, "subtrees")

			local := onet.NewLocalTest(testSuite)
			servers, roster, tree := local.GenTree(nNodes, false)

			require.NotNil(t, roster)

			// find first subtree leaves servers based on GenTree function
			leafsServerIdentities, err := GetLeafsIDs(tree, 0, nNodes, nSubtrees)
			if err != nil {
				t.Fatal(err)
			}
			failing := (len(leafsServerIdentities) - 1) / 3 // we render unresponsive one third of leafs
			threshold := nNodes - failing
			failingLeafsServerIdentities := leafsServerIdentities[:failing]
			firstLeavesServers := make([]*onet.Server, 0)
			for _, s := range servers {
				for _, l := range failingLeafsServerIdentities {
					if s.ServerIdentity.ID == l {
						firstLeavesServers = append(firstLeavesServers, s)
						break
					}
				}
			}

			// pause the router for the faulty servers
			for _, l := range firstLeavesServers {
				l.Pause()
			}

			services := local.GetServices(servers, testServiceID)
			// Share public keys among services
			sharePublicKeys(services)

			rootService := services[0].(*testService)
			pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}

			cosiProtocol := pi.(*BlsFtCosi)
			cosiProtocol.CreateProtocol = rootService.CreateProtocol
			cosiProtocol.Msg = proposal
			cosiProtocol.NSubtrees = nSubtrees
			cosiProtocol.Timeout = defaultTimeout
			cosiProtocol.Threshold = threshold

			cosiProtocol.PairingPrivate = rootService.private
			cosiProtocol.PairingPublic = rootService.public
			cosiProtocol.PairingPublics = rootService.pairingPublicKeys

			// start protocol
			err = cosiProtocol.Start()
			if err != nil {
				local.CloseAll()
				t.Fatal("error in starting of protocol:", err)
			}

			// get and verify signature
			_, err = getAndVerifySignature(cosiProtocol, proposal, NewThresholdPolicy(threshold))
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			}

			local.CloseAll()
		}
	}
}

// Tests unresponsive subleaders in various tree configurations
func TestUnresponsiveSubleader(t *testing.T) {
	nodes := []int{3, 13, 24}
	subtrees := []int{1, 2}
	proposal := []byte{0xFF}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			log.Lvl2("test asking for", nNodes, "nodes and", nSubtrees, "subtrees")

			local := onet.NewLocalTest(testSuite)
			servers, _, tree := local.GenTree(nNodes, false)

			// find first subleader server based on genTree function
			subleaderIds, err := GetSubleaderIDs(tree, 0, nNodes, nSubtrees)
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			} else if len(subleaderIds) < 1 {
				local.CloseAll()
				t.Fatal("found no subleader in generated tree with ", nNodes, "nodes and", nSubtrees, "subtrees")
			}
			var firstSubleaderServer *onet.Server
			for _, s := range servers {
				if s.ServerIdentity.ID == subleaderIds[0] {
					firstSubleaderServer = s
					break
				}
			}

			// pause the first sub leader to simulate failure
			firstSubleaderServer.Pause()

			services := local.GetServices(servers, testServiceID)
			// Share public keys among services
			sharePublicKeys(services)

			rootService := services[0].(*testService)
			pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}

			cosiProtocol := pi.(*BlsFtCosi)
			cosiProtocol.CreateProtocol = rootService.CreateProtocol
			cosiProtocol.Msg = proposal
			cosiProtocol.NSubtrees = nSubtrees
			cosiProtocol.Timeout = defaultTimeout
			cosiProtocol.Threshold = nNodes - 1

			cosiProtocol.PairingPrivate = rootService.private
			cosiProtocol.PairingPublic = rootService.public
			cosiProtocol.PairingPublics = rootService.pairingPublicKeys

			// start protocol
			err = cosiProtocol.Start()
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in starting of protocol:", err)
			}

			// get and verify signature
			_, err = getAndVerifySignature(cosiProtocol, proposal, NewThresholdPolicy(nNodes-1))
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			}

			local.CloseAll()
		}
	}
}

// Tests that the protocol throws errors with invalid configurations
func TestProtocolErrors(t *testing.T) {
	nodes := []int{1, 2, 5, 13, 24}
	subtrees := []int{1, 2, 5}
	proposal := []byte{0xFF}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			if nSubtrees >= nNodes && nSubtrees > 1 {
				continue
			}
			log.Lvl2("test asking for", nNodes, "nodes and", nSubtrees, "subtrees")

			local := onet.NewLocalTest(testSuite)
			servers, _, tree := local.GenTree(nNodes, false)

			services := local.GetServices(servers, testServiceID)
			// Share public keys among services
			sharePublicKeys(services)

			rootService := services[0].(*testService)
			pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}

			cosiProtocol := pi.(*BlsFtCosi)
			//cosiProtocol.CreateProtocol = rootService.CreateProtocol
			cosiProtocol.Msg = proposal
			cosiProtocol.NSubtrees = nSubtrees
			cosiProtocol.Timeout = defaultTimeout

			err = cosiProtocol.Start()
			if err == nil {
				local.CloseAll()
				t.Fatal("protocol should throw an error if called without create protocol function, but doesn't")
			}

			// missing proposal
			pi, err = local.CreateProtocol(DefaultProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}
			cosiProtocol = pi.(*BlsFtCosi)
			cosiProtocol.CreateProtocol = rootService.CreateProtocol
			//cosiProtocol.Msg = proposal
			cosiProtocol.NSubtrees = nSubtrees
			cosiProtocol.Timeout = defaultTimeout

			err = cosiProtocol.Start()
			if err == nil {
				local.CloseAll()
				t.Fatal("protocol should throw an error if called without a proposal, but doesn't")
			}

			local.CloseAll()
		}
	}
}

func TestProtocolRefusalAll(t *testing.T) {
	// TODO with 4 nodes passes, with 5 nodes and 1 subtree fails! (i.e. when there are 3 brother leaves)
	nodes := []int{4, 5, 13}
	subtrees := []int{1, 2, 5, 9}
	proposal := []byte{0xFF}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			if nSubtrees >= nNodes && nSubtrees > 1 {
				continue
			}
			log.Lvl2("test asking for", nNodes, "nodes and", nSubtrees, "subtrees")

			local := onet.NewLocalTest(testSuite)
			servers, _, tree := local.GenTree(nNodes, false)

			services := local.GetServices(servers, testServiceID)
			// Share public keys among services
			sharePublicKeys(services)

			rootService := services[0].(*testService)
			pi, err := rootService.CreateProtocol(FailureProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}

			cosiProtocol := pi.(*BlsFtCosi)
			cosiProtocol.CreateProtocol = rootService.CreateProtocol
			cosiProtocol.Msg = proposal
			cosiProtocol.NSubtrees = nSubtrees
			cosiProtocol.Timeout = defaultTimeout
			cosiProtocol.Threshold = nNodes / 2

			cosiProtocol.PairingPrivate = rootService.private
			cosiProtocol.PairingPublic = rootService.public
			cosiProtocol.PairingPublics = rootService.pairingPublicKeys

			err = cosiProtocol.Start()
			if err != nil {
				local.CloseAll()
				t.Fatal(err)
			}

			// only the leader agrees, the verification should only pass with a threshold of 1
			// the rest, including using the complete policy should fail
			var signature []byte
			select {
			case signature = <-cosiProtocol.FinalSignature:
				log.Lvl3("Instance is done")
			case <-time.After(defaultTimeout * 2):
				// wait a bit longer than the protocol timeout
				local.CloseAll()
				t.Fatal("didn't get signature in time")
			}

			require.Nil(t, signature)

			local.CloseAll()
		}
	}
}

func TestProtocolRefuseOne(t *testing.T) {
	nodes := []int{4, 5, 13}
	subtrees := []int{1, 2, 5, 9}
	proposal := []byte{0xFF}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			if nSubtrees >= nNodes && nSubtrees > 1 {
				continue
			}
			for refuseIdx := 1; refuseIdx < nNodes; refuseIdx++ {
				log.Lvl2("test asking for", nNodes, "nodes and", nSubtrees, "subtrees. "+
					"Node", refuseIdx, "will refuse.")
				counter = &Counter{refuseIdx: refuseIdx}

				local := onet.NewLocalTest(testSuite)

				servers, _, tree := local.GenTree(nNodes, false)

				services := local.GetServices(servers, testServiceID)
				// Share public keys among services
				sharePublicKeys(services)

				rootService := services[0].(*testService)
				pi, err := rootService.CreateProtocol(RefuseOneProtocolName, tree)
				if err != nil {
					local.CloseAll()
					t.Fatal("Error in creation of protocol:", err)
				}

				cosiProtocol := pi.(*BlsFtCosi)
				cosiProtocol.CreateProtocol = rootService.CreateProtocol
				cosiProtocol.Msg = proposal
				cosiProtocol.NSubtrees = nSubtrees
				cosiProtocol.Timeout = defaultTimeout
				cosiProtocol.Threshold = nNodes - 1

				cosiProtocol.PairingPrivate = rootService.private
				cosiProtocol.PairingPublic = rootService.public
				cosiProtocol.PairingPublics = rootService.pairingPublicKeys

				err = cosiProtocol.Start()
				if err != nil {
					local.CloseAll()
					t.Fatal(err)
				}

				// only the leader agrees, the verification should only pass with a threshold of 1
				// the rest, including using the complete policy should fail
				var signature []byte
				select {
				case signature = <-cosiProtocol.FinalSignature:
					log.Lvl3("Instance is done")
				case <-time.After(defaultTimeout * 2):
					// wait a bit longer than the protocol timeout
					local.CloseAll()
					t.Fatal("didn't get signature in time")
				}

				err = verifySignature(signature, cosiProtocol.PairingPublics, proposal, CompletePolicy{})
				if err == nil {
					local.CloseAll()
					t.Fatalf("verification should fail, refused index: %d", refuseIdx)
				}

				err = verifySignature(signature, cosiProtocol.PairingPublics, proposal, NewThresholdPolicy(nNodes-1))
				if err != nil {
					local.CloseAll()
					t.Fatal(err)
				}
				local.CloseAll()

				//TODO- The counter verification needs to be fixed for subtree regeneration
				/*
					counter.Lock()
					if counter.veriCount != nNodes-1 {
						counter.Unlock()
						t.Fatalf("not the right number of verified count, need %d but got %d", nNodes-1, counter.veriCount)
					}*/
			}
		}
	}
}

// testService allows setting the pairing keys of the protocol.
type testService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	// Has to be initialized by the test
	private           kyber.Scalar
	public            kyber.Point
	pairingPublicKeys []kyber.Point
}

func getAndVerifySignature(cosiProtocol *BlsFtCosi, proposal []byte, policy Policy) ([]byte, error) {
	var signature []byte
	log.Lvl3("Waiting for Instance")
	select {
	case signature = <-cosiProtocol.FinalSignature:
		log.Lvl3("Instance is done")
	case <-time.After(defaultTimeout * 2):
		// wait a bit longer than the protocol timeout
		log.Lvl3("Didnt received commitment in time")
		return nil, fmt.Errorf("didn't get commitment in time")
	}

	return signature, verifySignature(signature, cosiProtocol.PairingPublics, proposal, policy)
}

func verifySignature(signature []byte, publics []kyber.Point,
	proposal []byte, policy Policy) error {
	// verify signature
	err := Verify(pairingSuite, publics, proposal, signature, policy)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}
	log.Lvl3("Signature correctly verified!")
	return nil
}

type Counter struct {
	veriCount int
	refuseIdx int
	sync.Mutex
}

var counter = &Counter{}

func refuse(n *onet.TreeNodeInstance, msg, data []byte) bool {
	counter.Lock()
	defer counter.Unlock()
	defer func() { counter.veriCount++ }()
	if n.TreeNode().RosterIndex == counter.refuseIdx {
		return false
	}
	return true
}

// Starts a new service. No function needed.
func newService(c *onet.Context) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	s.private, s.public = bls.NewKeyPair(pairingSuite, random.New())
	return s, nil
}

// Shares the pairing public keys among services.
func sharePublicKeys(services []onet.Service) {
	publicKeys := make([]kyber.Point, len(services))
	for i, service := range services {
		tService := service.(*testService)
		publicKeys[i] = tService.public
	}

	for _, service := range services {
		tService := service.(*testService)
		tService.pairingPublicKeys = publicKeys
	}
}

// Store the public and private keys in the protocol
func (s *testService) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Cosi Service received New Protocol event")
	newProtocolMethod, ok := newProtocolMethods[tn.ProtocolName()]
	if !ok {
		return nil, errors.New("unknown protocol for this service")
	}

	pi, err := newProtocolMethod(tn)
	if err != nil {
		return nil, err
	}
	switch tn.ProtocolName() {
	case DefaultProtocolName, FailureProtocolName, RefuseOneProtocolName:
		blsftcosi := pi.(*BlsFtCosi)
		blsftcosi.PairingPrivate = s.private
		blsftcosi.PairingPublic = s.public
		blsftcosi.PairingPublics = s.pairingPublicKeys
		return blsftcosi, nil
	case DefaultSubProtocolName, FailureSubProtocolName, RefuseOneSubProtocolName:
		subblsftcosi := pi.(*SubBlsFtCosi)
		subblsftcosi.PairingPrivate = s.private
		subblsftcosi.PairingPublic = s.public
		subblsftcosi.PairingPublics = s.pairingPublicKeys
		return subblsftcosi, nil
	}
	return nil, errors.New("unknown protocol for this service")
}

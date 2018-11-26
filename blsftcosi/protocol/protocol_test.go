package protocol

import (
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"

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
	return NewBlsFtCosi(n, vf, FailureSubProtocolName, testSuite)
}
func NewFailureSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return false }
	return NewSubBlsFtCosi(n, vf, testSuite)
}

// Used for tests
var testServiceID onet.ServiceID

const testServiceName = "ServiceBlsFtCosi"

var newProtocolMethods = map[string]func(*onet.TreeNodeInstance) (onet.ProtocolInstance, error){
	DefaultProtocolName:    NewDefaultProtocol,
	DefaultSubProtocolName: NewDefaultSubProtocol,
	FailureProtocolName:    NewFailureProtocol,
	FailureSubProtocolName: NewFailureSubProtocol,
}

func init() {
	var err error
	testServiceID, err = onet.RegisterNewService(testServiceName, newService)
	log.ErrFatal(err)

	// Register Protocols
	GlobalRegisterDefaultProtocols()
	onet.GlobalProtocolRegister(FailureProtocolName, NewFailureProtocol)
	onet.GlobalProtocolRegister(FailureSubProtocolName, NewFailureSubProtocol)
}

var testSuite = bn256.NewSuiteG2()
var defaultTimeout = 2 * time.Second

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		defaultTimeout = 20 * time.Second
	}
	log.MainTest(m)
}

// Tests various trees configurations
func TestProtocol_1_1(t *testing.T) {
	_, _, err := runProtocol(1, 1, 1)
	require.Nil(t, err)
}

func TestProtocol_5_1(t *testing.T) {
	_, _, err := runProtocol(5, 1, 5)
	require.Nil(t, err)
}

func TestProtocol_25_1(t *testing.T) {
	_, _, err := runProtocol(25, 1, 25)
	require.Nil(t, err)
}

func TestProtocol_7_5(t *testing.T) {
	_, _, err := runProtocol(7, 5, 5)
	require.Nil(t, err)
}

func TestProtocol_25_5(t *testing.T) {
	_, _, err := runProtocol(25, 5, 25)
	require.Nil(t, err)
}

func runProtocol(nbrNodes, nbrSubTrees, threshold int) ([]byte, *onet.Roster, error) {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	servers, roster, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		return nil, nil, err
	}

	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{0xFF}
	cosiProtocol.NSubtrees = nbrSubTrees
	cosiProtocol.Timeout = defaultTimeout
	cosiProtocol.Threshold = threshold

	err = cosiProtocol.Start()
	if err != nil {
		return nil, nil, err
	}

	// get and verify signature
	sig, err := getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, NewThresholdPolicy(threshold))
	if err != nil {
		return nil, nil, err
	}

	return sig, roster, nil
}

func TestQuickAnswerProtocol_2_1(t *testing.T) {
	mask, err := runQuickAnswerProtocol(2, 1)
	require.Nil(t, err)
	require.Equal(t, 1, mask.CountEnabled())
}

func TestQuickAnswerProtocol_5_1(t *testing.T) {
	mask, err := runQuickAnswerProtocol(5, 1)
	require.Nil(t, err)
	require.Equal(t, 2, mask.CountEnabled())
}

func TestQuickAnswerProtocol_25_5(t *testing.T) {
	mask, err := runQuickAnswerProtocol(24, 5)
	require.Nil(t, err)
	require.InEpsilon(t, 12, mask.CountEnabled(), 5)
}

func runQuickAnswerProtocol(nbrNodes, nbrTrees int) (*Mask, error) {
	sig, roster, err := runProtocol(nbrNodes, nbrTrees, nbrNodes/2)
	if err != nil {
		return nil, err
	}

	mask, err := NewMask(testSuite, roster.Publics(), -1)
	if err != nil {
		return nil, err
	}
	lenRes := testSuite.G1().PointLen()
	mask.SetMask(sig[lenRes:])

	return mask, nil
}

func TestProtocol_FailingLeaves_5_1(t *testing.T) {
	err := runProtocolFailingNodes(5, 1, 1)
	require.Nil(t, err)
}

func TestProtocol_FailingLeaves_25_9(t *testing.T) {
	err := runProtocolFailingNodes(25, 3, 2)
	require.Nil(t, err)
}

func runProtocolFailingNodes(nbrNodes, nbrTrees, nbrFailure int) error {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)
	threshold := nbrNodes - nbrFailure

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		return err
	}

	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{0xFF}
	cosiProtocol.NSubtrees = nbrTrees
	cosiProtocol.Timeout = defaultTimeout
	cosiProtocol.Threshold = threshold

	leaves := cosiProtocol.getLeaves()
	for _, s := range servers {
		for _, l := range leaves[:nbrFailure] {
			if s.ServerIdentity.ID.Equal(l.ID) {
				s.Pause()
			}
		}
	}

	// start protocol
	err = cosiProtocol.Start()
	if err != nil {
		return err
	}

	// get and verify signature
	_, err = getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, NewThresholdPolicy(threshold))
	if err != nil {
		return err
	}

	return nil
}

func TestProtocol_FailingSubLeader_5_1(t *testing.T) {
	err := runProtocolFailingSubLeader(5, 1)
	require.Nil(t, err)
}

func TestProtocol_FailingSubLeader_25_3(t *testing.T) {
	err := runProtocolFailingSubLeader(25, 3)
	require.Nil(t, err)
}

func runProtocolFailingSubLeader(nbrNodes, nbrTrees int) error {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		return err
	}

	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{1, 2, 3}
	cosiProtocol.NSubtrees = nbrTrees
	cosiProtocol.Timeout = defaultTimeout
	cosiProtocol.Threshold = nbrNodes - 1

	subLeaders := cosiProtocol.getSubLeaders()
	for _, s := range servers {
		if s.ServerIdentity.ID.Equal(subLeaders[0].ID) {
			s.Pause()
		}
	}

	// start protocol
	err = cosiProtocol.Start()
	if err != nil {
		return err
	}

	// get and verify signature
	_, err = getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, NewThresholdPolicy(cosiProtocol.Threshold))
	if err != nil {
		return err
	}

	return nil
}

// Tests that the protocol throws errors with invalid configurations
func TestProtocol_IntegrityCheck(t *testing.T) {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenTree(1, false)

	services := local.GetServices(servers, testServiceID)

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		t.Fatal("Error in creation of protocol:", err)
	}

	// missing create protocol
	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.Msg = []byte{}
	cosiProtocol.NSubtrees = 1
	cosiProtocol.Timeout = defaultTimeout

	err = cosiProtocol.Start()
	if err == nil {
		t.Fatal("protocol should throw an error if called without create protocol function, but doesn't")
	}

	// missing proposal
	pi, err = local.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		t.Fatal("Error in creation of protocol:", err)
	}
	cosiProtocol = pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.NSubtrees = 1
	cosiProtocol.Timeout = defaultTimeout

	err = cosiProtocol.Start()
	if err == nil {
		t.Fatal("protocol should throw an error if called without a proposal, but doesn't")
	}
}

func TestProtocol_AllFailing_5_1(t *testing.T) {
	err := runProtocolAllFailing(5, 1, 1)
	require.Nil(t, err)

	err = runProtocolAllFailing(5, 1, 2)
	require.NotNil(t, err)
}

func TestProtocol_AllFailing_25_3(t *testing.T) {
	err := runProtocolAllFailing(25, 3, 1)
	require.Nil(t, err)

	err = runProtocolAllFailing(25, 3, 2)
	require.NotNil(t, err)
}

func runProtocolAllFailing(nbrNodes, nbrTrees, threshold int) error {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(FailureProtocolName, tree)
	if err != nil {
		return err
	}

	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{}
	cosiProtocol.NSubtrees = nbrTrees
	cosiProtocol.Timeout = defaultTimeout
	cosiProtocol.Threshold = threshold

	err = cosiProtocol.Start()
	if err != nil {
		return err
	}

	// only the leader agrees, the verification should only pass with a threshold of 1
	// the rest, including using the complete policy should fail
	_, err = getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, NewThresholdPolicy(threshold))
	if err != nil {
		return err
	}

	return nil
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

	return signature, verifySignature(signature, cosiProtocol.Roster().Publics(), proposal, policy)
}

func verifySignature(signature []byte, publics []kyber.Point,
	proposal []byte, policy Policy) error {
	// verify signature
	err := Verify(testSuite, publics, proposal, signature, policy)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}
	log.Lvl3("Signature correctly verified!")
	return nil
}

// Starts a new service. No function needed.
func newService(c *onet.Context) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	s.private, s.public = bls.NewKeyPair(testSuite, random.New())
	return s, nil
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
	case DefaultProtocolName, FailureProtocolName:
		blsftcosi := pi.(*BlsFtCosi)
		return blsftcosi, nil
	case DefaultSubProtocolName, FailureSubProtocolName:
		subblsftcosi := pi.(*SubBlsFtCosi)
		return subblsftcosi, nil
	}
	return nil, errors.New("unknown protocol for this service")
}

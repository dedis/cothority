package protocol

import (
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

const FailureProtocolName = "FailureProtocol"
const FailureSubProtocolName = "FailureSubProtocol"

func NewFailureProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBlsCosi(n, vf, FailureSubProtocolName, testSuite)
}
func NewFailureSubProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return false }
	return NewSubBlsCosi(n, vf, testSuite)
}

// Used for tests
var testServiceID onet.ServiceID

const testServiceName = "TestServiceBlsCosi"

var newProtocolMethods = map[string]func(*onet.TreeNodeInstance) (onet.ProtocolInstance, error){
	DefaultProtocolName:    NewDefaultProtocol,
	DefaultSubProtocolName: NewDefaultSubProtocol,
	FailureProtocolName:    NewFailureProtocol,
	FailureSubProtocolName: NewFailureSubProtocol,
}

func init() {
	var err error
	testServiceID, err = onet.RegisterNewServiceWithSuite(testServiceName, testSuite, newService)
	log.ErrFatal(err)

	// Register Protocols
	GlobalRegisterDefaultProtocols()
	onet.GlobalProtocolRegister(FailureProtocolName, NewFailureProtocol)
	onet.GlobalProtocolRegister(FailureSubProtocolName, NewFailureSubProtocol)
}

var testSuite = pairing.NewSuiteBn256()
var testTimeout = 20 * time.Second

func TestMain(m *testing.M) {
	flag.Parse()
	log.MainTest(m)
}

// Tests various trees configurations
func TestProtocol_1_1(t *testing.T) {
	_, _, err := runProtocol(1, 0, 1)
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
	_, _, err := runProtocol(7, 5, 7)
	require.Nil(t, err)
}

func TestProtocol_25_5(t *testing.T) {
	_, _, err := runProtocol(25, 5, 25)
	require.Nil(t, err)
}

func runProtocol(nbrNodes, nbrSubTrees, threshold int) (BlsSignature, *onet.Roster, error) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers, roster, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		return nil, nil, err
	}

	cosiProtocol := pi.(*BlsCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{0xFF}
	cosiProtocol.Timeout = testTimeout
	cosiProtocol.Threshold = threshold
	if nbrSubTrees > 0 {
		err = cosiProtocol.SetNbrSubTree(nbrSubTrees)
		if err != nil {
			return nil, nil, err
		}
	}

	err = cosiProtocol.Start()
	if err != nil {
		return nil, nil, err
	}

	// get and verify signature
	sig, err := getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, cosi.NewThresholdPolicy(threshold))
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

func TestQuickAnswerProtocol_5_4(t *testing.T) {
	mask, err := runQuickAnswerProtocol(5, 4)
	require.Nil(t, err)
	require.Equal(t, 2, mask.CountEnabled())
}

func TestQuickAnswerProtocol_24_5(t *testing.T) {
	mask, err := runQuickAnswerProtocol(24, 5)
	require.Nil(t, err)
	require.InEpsilon(t, 14, mask.CountEnabled(), 2)
}

func runQuickAnswerProtocol(nbrNodes, nbrTrees int) (*cosi.Mask, error) {
	sig, roster, err := runProtocol(nbrNodes, nbrTrees, nbrNodes/2)
	if err != nil {
		return nil, err
	}

	publics := roster.ServicePublics(testServiceName)

	mask, err := cosi.NewMask(testSuite, publics, nil)
	if err != nil {
		return nil, err
	}
	lenRes := testSuite.G1().PointLen()
	mask.SetMask(sig[lenRes:])

	return mask, nil
}

func TestProtocol_FailingLeaves_5_1(t *testing.T) {
	err := runProtocolFailingNodes(5, 1, 1, 4)
	require.Nil(t, err)
}

func TestProtocol_FailingLeaves_25_9(t *testing.T) {
	err := runProtocolFailingNodes(25, 3, 2, 23)
	require.Nil(t, err)
}

func runProtocolFailingNodes(nbrNodes, nbrTrees, nbrFailure, threshold int) error {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		return err
	}

	cosiProtocol := pi.(*BlsCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{0xFF}
	cosiProtocol.Timeout = testTimeout
	cosiProtocol.Threshold = threshold
	cosiProtocol.SetNbrSubTree(nbrTrees)

	leaves := cosiProtocol.subTrees.GetLeaves()
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
	_, err = getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, cosi.NewThresholdPolicy(threshold))
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

	cosiProtocol := pi.(*BlsCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{1, 2, 3}
	cosiProtocol.Timeout = testTimeout
	cosiProtocol.Threshold = nbrNodes - 1
	cosiProtocol.SetNbrSubTree(nbrTrees)

	subLeaders := cosiProtocol.subTrees.GetSubLeaders()
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
	_, err = getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, cosi.NewThresholdPolicy(cosiProtocol.Threshold))
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
	cosiProtocol := pi.(*BlsCosi)
	cosiProtocol.Msg = []byte{}
	cosiProtocol.Timeout = testTimeout

	// wrong subtree number
	err = cosiProtocol.SetNbrSubTree(1)
	require.NotNil(t, err)

	err = cosiProtocol.Start()
	if err == nil {
		t.Fatal("protocol should throw an error if called without create protocol function, but doesn't")
	}

	// missing proposal
	pi, err = local.CreateProtocol(DefaultProtocolName, tree)
	if err != nil {
		t.Fatal("Error in creation of protocol:", err)
	}
	cosiProtocol = pi.(*BlsCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Timeout = testTimeout
	cosiProtocol.SetNbrSubTree(1)

	err = cosiProtocol.Start()
	if err == nil {
		t.Fatal("protocol should throw an error if called without a proposal, but doesn't")
	}
}

func TestProtocol_AllFailing_5_1(t *testing.T) {
	_, err := runProtocolAllFailing(5, 1, 1)
	require.Nil(t, err)

	_, err = runProtocolAllFailing(5, 1, 2)
	require.NotNil(t, err)
}

func TestProtocol_AllFailing_25_3(t *testing.T) {
	_, err := runProtocolAllFailing(25, 3, 1)
	require.Nil(t, err)

	_, err = runProtocolAllFailing(25, 3, 2)
	require.NotNil(t, err)
}

func TestProtocol_QuickFailure_15(t *testing.T) {
	ts, err := runProtocolAllFailing(15, 1, 15)
	require.NotNil(t, err)
	require.True(t, ts.Add(testTimeout).After(time.Now()), "Protocol should not reach the timeout")
}

func TestProtocol_QuickFailure_14(t *testing.T) {
	ts, err := runProtocolAllFailing(15, 1, 14)
	require.NotNil(t, err)
	require.True(t, ts.Add(testTimeout).After(time.Now()), "Protocol should not reach the timeout")
}

func runProtocolAllFailing(nbrNodes, nbrTrees, threshold int) (time.Time, error) {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)

	time := time.Now()
	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(FailureProtocolName, tree)
	if err != nil {
		return time, err
	}

	cosiProtocol := pi.(*BlsCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{}
	cosiProtocol.Timeout = testTimeout
	cosiProtocol.Threshold = threshold
	cosiProtocol.SetNbrSubTree(nbrTrees)

	err = cosiProtocol.Start()
	if err != nil {
		return time, err
	}

	// only the leader agrees, the verification should only pass with a threshold of 1
	// the rest, including using the complete policy should fail
	_, err = getAndVerifySignature(cosiProtocol, cosiProtocol.Msg, cosi.NewThresholdPolicy(threshold))
	if err != nil {
		return time, err
	}

	return time, nil
}

// testService allows setting the pairing keys of the protocol.
type testService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
}

func getAndVerifySignature(proto *BlsCosi, proposal []byte, policy cosi.Policy) (BlsSignature, error) {
	var signature BlsSignature
	log.Lvl3("Waiting for Instance")
	select {
	case signature = <-proto.FinalSignature:
		log.Lvl3("Instance is done")
	case <-time.After(testTimeout * 2):
		// wait a bit longer than the protocol timeout
		log.Lvl3("Didnt received commitment in time")
		return nil, fmt.Errorf("didn't get commitment in time")
	}

	publics := proto.Roster().ServicePublics(testServiceName)
	return signature, signature.VerifyWithPolicy(testSuite, proto.Msg, publics, policy)
}

// Starts a new service. No function needed.
func newService(c *onet.Context) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
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
		blscosi := pi.(*BlsCosi)
		return blscosi, nil
	case DefaultSubProtocolName, FailureSubProtocolName:
		subblscosi := pi.(*SubBlsCosi)
		return subblscosi, nil
	}
	return nil, errors.New("unknown protocol for this service")
}

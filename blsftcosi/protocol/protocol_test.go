package protocol

import (
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
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
	_, _, err := runProtocol(7, 5, 5)
	require.Nil(t, err)
}

func TestProtocol_25_5(t *testing.T) {
	_, _, err := runProtocol(25, 5, 25)
	require.Nil(t, err)
}

func runProtocol(nbrNodes, nbrSubTrees, threshold int) (BlsSignature, *onet.Roster, error) {
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
	cosiProtocol.Timeout = defaultTimeout
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

func TestQuickAnswerProtocol_5_1(t *testing.T) {
	mask, err := runQuickAnswerProtocol(5, 1)
	require.Nil(t, err)
	require.Equal(t, 3, mask.CountEnabled())
}

func TestQuickAnswerProtocol_25_5(t *testing.T) {
	mask, err := runQuickAnswerProtocol(24, 5)
	require.Nil(t, err)
	require.InEpsilon(t, 12, mask.CountEnabled(), 5)
}

func runQuickAnswerProtocol(nbrNodes, nbrTrees int) (*cosi.Mask, error) {
	sig, roster, err := runProtocol(nbrNodes, nbrTrees, nbrNodes/2)
	if err != nil {
		return nil, err
	}

	var suite network.Suite
	suite = testSuite
	mask, err := cosi.NewMask(suite.(cosi.Suite), roster.Publics(), nil)
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

	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{0xFF}
	cosiProtocol.Timeout = defaultTimeout
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

	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{1, 2, 3}
	cosiProtocol.Timeout = defaultTimeout
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
	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.Msg = []byte{}
	cosiProtocol.Timeout = defaultTimeout

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
	cosiProtocol = pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Timeout = defaultTimeout
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
	require.True(t, ts.Add(defaultTimeout).After(time.Now()), "Protocol should not reach the timeout")
}

func TestProtocol_QuickFailure_14(t *testing.T) {
	ts, err := runProtocolAllFailing(15, 1, 14)
	require.NotNil(t, err)
	require.True(t, ts.Add(defaultTimeout).After(time.Now()), "Protocol should not reach the timeout")
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

	cosiProtocol := pi.(*BlsFtCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{}
	cosiProtocol.Timeout = defaultTimeout
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

func getAndVerifySignature(proto *BlsFtCosi, proposal []byte, policy cosi.Policy) (BlsSignature, error) {
	var signature BlsSignature
	log.Lvl3("Waiting for Instance")
	select {
	case signature = <-proto.FinalSignature:
		log.Lvl3("Instance is done")
	case <-time.After(defaultTimeout * 2):
		// wait a bit longer than the protocol timeout
		log.Lvl3("Didnt received commitment in time")
		return nil, fmt.Errorf("didn't get commitment in time")
	}

	return signature, signature.Verify(testSuite, proto.Msg, proto.Roster().Publics(), policy)
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
		blsftcosi := pi.(*BlsFtCosi)
		return blsftcosi, nil
	case DefaultSubProtocolName, FailureSubProtocolName:
		subblsftcosi := pi.(*SubBlsFtCosi)
		return subblsftcosi, nil
	}
	return nil, errors.New("unknown protocol for this service")
}

package omnicon

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/cosi/protocol"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

var testSuite = cothority.Suite

func TestMain(m *testing.M) {
	log.MainTest(m)
}

type testState struct {
	// xs is a binary array, 0 represents fail and 1 represents pass
	// this is the only field that should be initialised
	xs          []bool
	refuseCount int
	acceptCount int
	stateMut    sync.Mutex
}

func genVerificationFn(i int, s *testState) protocol.VerificationFn {
	return func(a []byte) bool {
		s.stateMut.Lock()
		defer s.stateMut.Unlock()
		if s.xs[i] {
			s.acceptCount++
			return true
		}
		s.refuseCount++
		return false
	}
}

func TestBftCoSi(t *testing.T) {
	const protoName = "TestBftCoSi"

	vf := func(a []byte) bool { return true }
	err := GlobalInitBFTCoSiProtocol(vf, protoName)
	require.Nil(t, err)
	err = runProtocol(t, 5, 0, protoName, []byte("hello world"))
	require.Nil(t, err)
}

func runProtocol(t *testing.T, nbrHosts int, nbrFault int, protoName string, proposal []byte) error {
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()

	_, _, tree := local.GenTree(nbrHosts, false)

	// get public keys
	publics := make([]kyber.Point, tree.Size())
	for i, node := range tree.List() {
		publics[i] = node.ServerIdentity.Public
	}

	pi, err := local.CreateProtocol(protoName, tree)
	require.Nil(t, err)

	bftCosiProto := pi.(*ProtocolBFTCoSi)
	bftCosiProto.CreateProtocol = local.CreateProtocol
	bftCosiProto.Proposal = proposal
	// TODO other fields?

	err = bftCosiProto.Start()
	require.Nil(t, err)

	var policy cosi.Policy
	if nbrFault == 0 {
		policy = nil
	} else {
		policy = cosi.NewThresholdPolicy(nbrFault)
	}
	return getAndVerifySignature(bftCosiProto.FinalSignature, publics, proposal, policy)
}

func getAndVerifySignature(sigChan chan []byte, publics []kyber.Point, proposal []byte, policy cosi.Policy) error {
	timeout := time.Second * 5
	var sig []byte
	select {
	case sig = <-sigChan:
	case <-time.After(timeout):
		return fmt.Errorf("didn't get commitment after a timeout of %v", timeout)
	}

	// verify signature
	if sig == nil {
		return fmt.Errorf("signature is nil")
	}
	err := cosi.Verify(testSuite, publics, proposal, sig, policy)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}
	log.Lvl2("Signature correctly verified!")
	return nil
}

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
	log.MainTest(m, 3)
}

type testState struct {
	sync.Mutex
	refuseCount int
	acceptCount int
	ackCount    int
}

func genVerificationFn(n, t int, s *testState) protocol.VerificationFn {
	return func(a []byte) bool {
		s.Lock()
		defer s.Unlock()
		// total number of calls = s.acceptCount+s.refuseCount
		log.Print(s.acceptCount+s.refuseCount, n, t)
		if s.acceptCount+s.refuseCount < n-t {
			s.acceptCount++
			return true
		}
		s.refuseCount++
		return false
	}
}

func genAckFn(s *testState) protocol.VerificationFn {
	return func(a []byte) bool {
		s.Lock()
		s.ackCount++
		s.Unlock()
		return true
	}
}

func TestBftCoSi(t *testing.T) {
	const protoName = "TestBftCoSi"

	s := testState{}
	vf := genVerificationFn(5, 0, &s)
	ack := genAckFn(&s)
	err := GlobalInitBFTCoSiProtocol(vf, ack, protoName)

	require.Nil(t, err)
	err = runProtocol(t, 5, 0, 0, protoName, []byte("hello world"))
	require.Nil(t, err)

	require.Equal(t, s.acceptCount, 5)
	require.Equal(t, s.ackCount, 5)
	require.Equal(t, s.refuseCount, 0)
}

func runProtocol(t *testing.T, nbrHosts int, nbrFault int, nbrRefuse int, protoName string, proposal []byte) error {
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
	h := testSuite.Hash()
	h.Write(proposal)
	err := cosi.Verify(testSuite, publics, h.Sum(nil), sig, policy)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}
	log.Lvl2("Signature correctly verified!")
	return nil
}

package eventlog

import (
	"testing"
	"time"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2/suites"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m, 3)
}

func TestService_Init(t *testing.T) {
	s := newSer(t)
	defer s.close()

	// With no signer: error
	_, err := s.services[0].Init(&InitRequest{})
	require.NotNil(t, err)

	// Do the initialisation
	s.init(t)
}

func TestService_Log(t *testing.T) {
	s := newSer(t)
	defer s.close()

	scID, d, signers := s.init(t)

	req := NewEvent("auth", "login")
	ctx, err := makeTx([]Event{req}, d.GetBaseID(), signers)
	require.Nil(t, err)

	_, err = s.services[0].Log(&LogRequest{
		SkipchainID: scID,
		Transaction: *ctx,
	})
	require.Nil(t, err)

	s.check(t, "one log arrived", func() bool {
		resp, err := s.services[0].omni.GetProof(&omniledger.GetProof{
			Version: omniledger.CurrentVersion,
			Key:     ctx.Instructions[0].ObjectID.Slice(),
			ID:      scID,
		})
		require.Nil(t, err)
		return resp.Proof.InclusionProof.Match()
	})
}

func (s *ser) init(t *testing.T) (skipchain.SkipBlockID, darc.Darc, []*darc.Signer) {
	owner := darc.NewSignerEd25519(nil, nil)
	rules := darc.InitRules([]*darc.Identity{owner.Identity()}, []*darc.Identity{})
	d1 := darc.NewDarc(AddWriter(rules, nil), []byte("eventlog writer"))

	reply, err := s.services[0].Init(&InitRequest{
		Roster: *s.roster,
		Owner:  *d1,
	})
	require.Nil(t, err)
	require.NotNil(t, reply.ID)
	require.False(t, reply.ID.IsNull())

	return reply.ID, *d1, []*darc.Signer{owner}
}

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
}

func (s *ser) close() {
	s.local.CloseAll()
	for _, x := range s.services {
		close(x.omni.CloseQueues)
	}
}

func (s *ser) check(t *testing.T, what string, f func() bool) {
	for ct := 0; ct < 10; ct++ {
		if f() == true {
			return
		}
		t.Log("check failed, sleep and retry")
		s.services[0].waitForBlock()
	}
	t.Fatalf("check for %v failed", what)
}

func newSer(t *testing.T) *ser {
	s := &ser{
		local: onet.NewTCPTest(tSuite),
	}
	s.hosts, s.roster, _ = s.local.GenTree(2, true)

	for _, sv := range s.local.GetServices(s.hosts, sid) {
		service := sv.(*Service)
		service.blockInterval = time.Second
		s.services = append(s.services, service)
	}

	return s
}

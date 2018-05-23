package eventlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/kyber.v2/suites"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_Init(t *testing.T) {
	s := newSer(t)
	defer s.local.CloseAll()

	_, err := s.services[0].Init(&InitRequest{})

	// We know it does not pass yet.
	require.NotNil(t, err)
}

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
}

func newSer(t *testing.T) *ser {
	s := &ser{
		local: onet.NewTCPTest(tSuite),
	}
	s.hosts, s.roster, _ = s.local.GenTree(2, true)

	for _, sv := range s.local.GetServices(s.hosts, sid) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}

	return s
}

package service

import (
	"testing"

	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2/suites"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateSkipchain(t *testing.T) {
	s := newSer(t, 0)
	defer s.local.CloseAll()
	resp, err := s.service.CreateSkipchain(&CreateSkipchain{
		Version: 0,
		Roster:  *s.roster,
	})
	require.NotNil(t, err)

	resp, err = s.service.CreateSkipchain(&CreateSkipchain{
		Version: CurrentVersion,
		Roster:  *s.roster,
	})
	require.Nil(t, err)
	assert.Equal(t, CurrentVersion, resp.Version)
	assert.NotNil(t, resp.Skipblock)
}

func TestService_AddKeyValue(t *testing.T) {
	s := newSer(t, 1)
	defer s.local.CloseAll()

	akvresp, err := s.service.SetKeyValue(&SetKeyValue{
		Version: 0,
	})
	require.NotNil(t, err)
	akvresp, err = s.service.SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Key:         s.key,
		Value:       s.value,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)
	require.NotNil(t, akvresp.Timestamp)
	require.NotNil(t, akvresp.SkipblockID)
}

func TestService_GetValue(t *testing.T) {
	s := newSer(t, 2)
	defer s.local.CloseAll()

	rep, err := s.service.GetValue(&GetValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Key:         s.key,
	})
	require.Nil(t, err)
	require.Equal(t, s.value, *rep.Value)
}

type ser struct {
	local   *onet.LocalTest
	hosts   []*onet.Server
	roster  *onet.Roster
	service *Service
	sb      *skipchain.SkipBlock
	key     []byte
	value   []byte
}

func newSer(t *testing.T, step int) *ser {
	s := &ser{
		local: onet.NewTCPTest(tSuite),
		key:   []byte("anykey"),
		value: []byte("anyvalue"),
	}
	s.hosts, s.roster, _ = s.local.GenTree(5, true)
	s.service = s.local.GetServices(s.hosts, lleapID)[0].(*Service)

	for i := 0; i < step; i++ {
		switch i {
		case 0:
			resp, err := s.service.CreateSkipchain(&CreateSkipchain{
				Version: CurrentVersion,
				Roster:  *s.roster,
			})
			assert.Nil(t, err)
			s.sb = resp.Skipblock
		case 1:
			_, err := s.service.SetKeyValue(&SetKeyValue{
				Version:     CurrentVersion,
				SkipchainID: s.sb.SkipChainID(),
				Key:         s.key,
				Value:       s.value,
			})
			assert.Nil(t, err)
		}
	}
	return s
}

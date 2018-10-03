package omniledger

import (
	"fmt"
	"github.com/dedis/kyber/suites"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"
	"testing"
	"time"

	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

var tSuite = suites.MustFind("Ed25519")
var testInterval = 500 * time.Millisecond
var shardCount = 1

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
	sb       *skipchain.SkipBlock
	value    []byte
	darc     *darc.Darc
	signer   darc.Signer
	tx       bc.ClientTransaction
	interval time.Duration
}

func TestService_CreateOmniLedger(t *testing.T) {
	s := newSer(t, 0, testInterval)
	defer s.local.CloseAll()

	// Check roster
	rep, err := s.service().CreateOmniLedger(&CreateOmniLedger{
		Version: bc.CurrentVersion,
	})
	require.NotNil(t, err)

	// Check #shard
	rep, err = s.service().CreateOmniLedger(&CreateOmniLedger{
		Version: bc.CurrentVersion,
		Roster:  *s.roster,
	})
	require.NotNil(t, err)

	// Check epoch size
	rep, err = s.service().CreateOmniLedger(&CreateOmniLedger{
		Version:    bc.CurrentVersion,
		Roster:     *s.roster,
		ShardCount: 1,
	})
	require.NotNil(t, err)

	// Check there is enough validator
	rep, err = s.service().CreateOmniLedger(&CreateOmniLedger{
		Version:    bc.CurrentVersion,
		Roster:     *s.roster,
		ShardCount: 5 * len(s.roster.List),
		EpochSize:  1,
	})
	require.NotNil(t, err)

	// Passing
	rep, err = s.service().CreateOmniLedger(&CreateOmniLedger{
		Version:    bc.CurrentVersion,
		Roster:     *s.roster,
		ShardCount: 1,
		EpochSize:  1,
	})

	log.Println(err)
	fmt.Println(rep)

	assert.NotNil(t, rep)
	assert.True(t, len(rep.ShardRoster) == shardCount)
}

func newSer(t *testing.T, step int, interval time.Duration) *ser {
	return newSerN(t, step, interval, 4, false)
}

func newSerN(t *testing.T, step int, interval time.Duration, n int, viewchange bool) *ser {
	s := &ser{
		local:  onet.NewLocalTestT(tSuite, t),
		value:  []byte("anyvalue"),
		signer: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(n, true)
	for _, sv := range s.local.GetServices(s.hosts, OmniLedgerID) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}

	return s
}

func (s *ser) service() *Service {
	return s.services[0]
}

package service

import (
	"bytes"
	"testing"
	"time"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2/suites"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	waitQueueing = 100 * time.Millisecond
	log.MainTest(m)
}

func TestService_CreateSkipchain(t *testing.T) {
	s := newSer(t, 0)
	defer s.local.CloseAll()
	defer closeQueues(s.local)
	resp, err := s.service.CreateSkipchain(&CreateSkipchain{
		Version: 0,
		Roster:  *s.roster,
	})
	require.NotNil(t, err)

	resp, err = s.service.CreateSkipchain(&CreateSkipchain{
		Version: CurrentVersion,
		Roster:  *s.roster,
		Transaction: Transaction{
			Key:   []byte("someKey"),
			Kind:  []byte("someKind"),
			Value: []byte("someValue"),
		},
	})
	require.Nil(t, err)
	assert.Equal(t, CurrentVersion, resp.Version)
	assert.NotNil(t, resp.Skipblock)
}

func TestService_AddKeyValue(t *testing.T) {
	s := newSer(t, 1)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	akvresp, err := s.service.SetKeyValue(&SetKeyValue{
		Version: 0,
	})
	require.NotNil(t, err)
	akvresp, err = s.service.SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: Transaction{
			Key:   s.key,
			Kind:  []byte("testKind"),
			Value: s.value,
		},
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	key2 := []byte("second")
	value2 := []byte("value2")
	akvresp, err = s.service.SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: Transaction{
			Key:   key2,
			Kind:  []byte("testKind"),
			Value: value2,
		},
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	pairs := map[string][]byte{string(s.key): s.value, string(key2): value2}

	for i := 0; i < 2; i++ {
		if i == 1 {
			// Now read the key/values from a new service
			log.Lvl1("Recreate services and fetch keys again")
			s.service.tryLoad()
		}
		for key, value := range pairs {
			for {
				time.Sleep(2 * waitQueueing)
				pr, err := s.service.GetProof(&GetProof{
					Version: CurrentVersion,
					ID:      s.sb.SkipChainID(),
					Key:     []byte(key),
				})
				if err != nil {
					continue
				}
				require.Equal(t, CurrentVersion, pr.Version)
				require.Nil(t, pr.Proof.Verify(s.sb.SkipChainID()))
				if pr.Proof.InclusionProof.Match() {
					_, vs, err := pr.Proof.KeyValue()
					require.Nil(t, err)
					require.Equal(t, 0, bytes.Compare(value, vs[0]))
					break
				}
			}
		}
	}
}

func TestService_GetProof(t *testing.T) {
	s := newSer(t, 2)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	var rep *GetProofResponse
	var i int
	for i = 0; i < 10; i++ {
		time.Sleep(2 * waitQueueing)
		var err error
		rep, err = s.service.GetProof(&GetProof{
			Version: CurrentVersion,
			ID:      s.sb.SkipChainID(),
			Key:     s.key,
		})
		require.Nil(t, err)
		if rep.Proof.InclusionProof.Match() {
			break
		}
	}
	require.NotEqual(t, 10, i, "didn't get proof in time")
	key, values, err := rep.Proof.KeyValue()
	require.Nil(t, err)
	require.Nil(t, rep.Proof.Verify(s.sb.SkipChainID()))
	require.Equal(t, s.key, key)
	require.Equal(t, s.value, values[0])

	rep, err = s.service.GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     append(s.key, byte(0)),
	})
	require.Nil(t, err)
	require.Nil(t, rep.Proof.Verify(s.sb.SkipChainID()))
	key, values, err = rep.Proof.KeyValue()
	require.NotNil(t, err)
}

func TestService_DummyVerification(t *testing.T) {
	s := newSer(t, 1)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	RegisterVerification(s.hosts[0], "dummy", verifyDummyKind)
	akvresp, err := s.service.SetKeyValue(&SetKeyValue{
		Version: 0,
	})
	require.NotNil(t, err)

	key1 := []byte("a")
	value1 := []byte("a")
	akvresp, err = s.service.SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: Transaction{
			Key:    key1,
			Kind:   []byte("dummy"),
			Value:  value1,
			Action: Remove,
		},
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	time.Sleep(2 * waitQueueing)
	pr, err := s.service.GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     key1,
	})
	require.Nil(t, err)
	match := pr.Proof.InclusionProof.Match()
	require.False(t, match)

	key2 := []byte("b")
	value2 := []byte("b")
	akvresp, err = s.service.SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: Transaction{
			Key:    key2,
			Kind:   []byte("other"),
			Value:  value2,
			Action: Remove,
		},
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	time.Sleep(4 * waitQueueing)
	pr, err = s.service.GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     key2,
	})
	require.Nil(t, err)
	match = pr.Proof.InclusionProof.Match()
	require.True(t, match)

	time.Sleep(4 * waitQueueing)
}

func verifyDummyKind(cdb *collectionDB, tx *Transaction) bool {
	switch a := tx.Action; a {
	case Create:
		return true
	case Update:
		return true
	// removing and unknown actions are forbidden
	case Remove:
		return false
	default:
		return false
	}
}

type ser struct {
	local   *onet.LocalTest
	hosts   []*onet.Server
	roster  *onet.Roster
	service *Service
	sb      *skipchain.SkipBlock
	key     []byte
	value   []byte
	darc    *darc.Darc
}

func newSer(t *testing.T, step int) *ser {
	s := &ser{
		local: onet.NewTCPTest(tSuite),
		key:   []byte("anykey"),
		value: []byte("anyvalue"),
	}
	s.hosts, s.roster, _ = s.local.GenTree(5, true)
	s.service = s.local.GetServices(s.hosts, omniledgerID)[0].(*Service)
	s.darc = &darc.Darc{}

	for i := 0; i < step; i++ {
		switch i {
		case 0:
			d, err := network.Marshal(s.darc)
			require.Nil(t, err)
			resp, err := s.service.CreateSkipchain(&CreateSkipchain{
				Version: CurrentVersion,
				Roster:  *s.roster,
				Transaction: Transaction{
					Key:   s.darc.GetID(),
					Kind:  []byte("darc"),
					Value: d,
				},
			})
			require.Nil(t, err)
			s.sb = resp.Skipblock
		case 1:
			_, err := s.service.SetKeyValue(&SetKeyValue{
				Version:     CurrentVersion,
				SkipchainID: s.sb.SkipChainID(),
				Transaction: Transaction{
					Key:   s.key,
					Kind:  []byte("testKind"),
					Value: s.value,
				},
			})
			require.Nil(t, err)
			time.Sleep(4 * waitQueueing)
		}
	}
	return s
}

func closeQueues(local *onet.LocalTest) {
	for _, server := range local.Servers {
		services := local.GetServices([]*onet.Server{server}, omniledgerID)
		close(services[0].(*Service).CloseQueues)
	}
}

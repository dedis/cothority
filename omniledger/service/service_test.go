package service

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2/suites"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

var tSuite = suites.MustFind("Ed25519")
var dummyKind = "dummy"

func TestMain(m *testing.M) {
	waitQueueing = 100 * time.Millisecond
	log.MainTest(m)
}

func TestService_CreateSkipchain(t *testing.T) {
	s := newSer(t, 0)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	// invalid version, missing transaction
	resp, err := s.service().CreateGenesisBlock(&CreateGenesisBlock{
		Version: 0,
		Roster:  *s.roster,
	})
	require.NotNil(t, err)

	// invalid darc
	resp, err = s.service().CreateGenesisBlock(&CreateGenesisBlock{
		Version:     CurrentVersion,
		Roster:      *s.roster,
		GenesisDarc: darc.Darc{},
	})
	require.NotNil(t, err)

	// create valid darc
	signer := darc.NewSignerEd25519(nil, nil)
	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster, []string{"Spawn_dummy"}, signer.Identity())
	require.Nil(t, err)

	// finally passing
	resp, err = s.service().CreateGenesisBlock(genesisMsg)
	require.Nil(t, err)
	assert.Equal(t, CurrentVersion, resp.Version)
	assert.NotNil(t, resp.Skipblock)
}

func padDarc(key []byte) []byte {
	keyPadded := make([]byte, 32)
	copy(keyPadded, key)
	return keyPadded
}

func TestService_AddKeyValue(t *testing.T) {
	s := newSer(t, 1)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	// wrong version
	akvresp, err := s.service().SetKeyValue(&SetKeyValue{
		Version: CurrentVersion + 1,
	})
	require.NotNil(t, err)

	// missing skipchain
	akvresp, err = s.service().SetKeyValue(&SetKeyValue{
		Version: CurrentVersion,
	})
	require.NotNil(t, err)

	// missing transaction
	akvresp, err = s.service().SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
	})
	require.NotNil(t, err)

	// the operations below should succeed
	// add the first tx
	tx1, err := createOneClientTx(s.darc.GetBaseID(), dummyKind, s.value, s.signer)
	require.Nil(t, err)
	akvresp, err = s.service().SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: tx1,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// add the second tx
	value2 := []byte("value2")
	tx2, err := createOneClientTx(s.darc.GetBaseID(), dummyKind, value2, s.signer)
	require.Nil(t, err)
	akvresp, err = s.service().SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: tx2,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// try to read the transaction back again
	txs := []ClientTransaction{tx1, tx2}
	for i := 0; i < 2; i++ {
		if i == 1 {
			// Now read the key/values from a new service
			log.Lvl1("Recreate services and fetch keys again")
			s.service().tryLoad()
		}
		for _, tx := range txs {
			for {
				time.Sleep(2 * waitQueueing)
				pr, err := s.service().GetProof(&GetProof{
					Version: CurrentVersion,
					ID:      s.sb.SkipChainID(),
					Key:     tx.Instructions[0].ObjectID.Slice(),
				})
				if err != nil {
					log.Error(err)
					continue
				}
				require.Equal(t, CurrentVersion, pr.Version)
				require.Nil(t, pr.Proof.Verify(s.sb.SkipChainID()))
				if pr.Proof.InclusionProof.Match() {
					_, vs, err := pr.Proof.KeyValue()
					require.Nil(t, err)
					require.Equal(t, 0, bytes.Compare(tx.Instructions[0].Spawn.Args[0].Value, vs[0]))
					break
				} else {
				}
			}
		}
	}
}

func TestService_GetProof(t *testing.T) {
	s := newSer(t, 2)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	serKey := s.tx.Instructions[0].ObjectID.Slice()

	var rep *GetProofResponse
	var i int
	for i = 0; i < 10; i++ {
		time.Sleep(2 * waitQueueing)
		var err error
		rep, err = s.service().GetProof(&GetProof{
			Version: CurrentVersion,
			ID:      s.sb.SkipChainID(),
			Key:     serKey,
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
	require.Equal(t, serKey, key)
	require.Equal(t, s.value, values[0])

	// Modify the key and we should not be able to get the proof.
	rep, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     append(serKey, byte(0)),
	})
	require.Nil(t, err)
	require.Nil(t, rep.Proof.Verify(s.sb.SkipChainID()))
	key, values, err = rep.Proof.KeyValue()
	require.NotNil(t, err)
}

func TestService_InvalidVerification(t *testing.T) {
	s := newSer(t, 1)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	for i := range s.hosts {
		RegisterContract(s.hosts[i], "invalid", verifyInvalidKind)
	}

	// tx1 uses the invalid kind, so it should _not_ be stored.
	value1 := []byte("a")
	tx1, err := createOneClientTx(s.darc.GetBaseID(), "invalid", value1, s.signer)
	require.Nil(t, err)
	akvresp, err := s.service().SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: tx1,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// tx2 uses the dummy kind, its value should be stored.
	value2 := []byte("b")
	tx2, err := createOneClientTx(s.darc.GetBaseID(), dummyKind, value2, s.signer)
	akvresp, err = s.service().SetKeyValue(&SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: tx2,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	time.Sleep(8 * waitQueueing)

	// Check that tx1 is _not_ stored.
	pr, err := s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     tx1.Instructions[0].ObjectID.Slice(),
	})
	require.Nil(t, err)
	match := pr.Proof.InclusionProof.Match()
	require.False(t, match)

	// Check that tx2 is stored.
	pr, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     tx2.Instructions[0].ObjectID.Slice(),
	})
	require.Nil(t, err)
	match = pr.Proof.InclusionProof.Match()
	require.True(t, match)
}

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
	sb       *skipchain.SkipBlock
	value    []byte
	darc     *darc.Darc
	signer   *darc.Signer
	tx       ClientTransaction
}

func (s *ser) service() *Service {
	return s.services[0]
}

func newSer(t *testing.T, step int) *ser {
	s := &ser{
		local:  onet.NewTCPTest(tSuite),
		value:  []byte("anyvalue"),
		signer: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(2, true)

	for _, sv := range s.local.GetServices(s.hosts, omniledgerID) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}
	registerDummy(s.services)

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster, []string{"Spawn_dummy", "Spawn_invalid"}, s.signer.Identity())
	require.Nil(t, err)
	s.darc = &genesisMsg.GenesisDarc

	for i := 0; i < step; i++ {
		switch i {
		case 0:
			resp, err := s.service().CreateGenesisBlock(genesisMsg)
			require.Nil(t, err)
			s.sb = resp.Skipblock
		case 1:
			tx, err := createOneClientTx(s.darc.GetBaseID(), dummyKind, s.value, s.signer)
			require.Nil(t, err)
			s.tx = tx
			_, err = s.service().SetKeyValue(&SetKeyValue{
				Version:     CurrentVersion,
				SkipchainID: s.sb.SkipChainID(),
				Transaction: tx,
			})
			require.Nil(t, err)
			time.Sleep(4 * waitQueueing)
		default:
			require.Fail(t, "no such step")
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

func verifyInvalidKind(cdb collection.Collection, tx Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return nil, nil, errors.New("Invalid")
}

func verifyDummy(cdb collection.Collection, tx Instruction, c []Coin) ([]StateChange, []Coin, error) {
	args := tx.Spawn.Args[0].Value
	cid, _, err := tx.GetContractState(cdb)
	if err != nil {
		return nil, nil, err
	}
	return []StateChange{
		NewStateChange(Create, tx.ObjectID, cid, args),
	}, nil, nil
}

func registerDummy(services interface{}) {
	// For testing - there must be a better way to do that. But putting
	// services []skipchain.GetService in the method signature doesn't work :(
	for i := 0; i < reflect.ValueOf(services).Len(); i++ {
		s := reflect.ValueOf(services).Index(i).Interface().(skipchain.GetService)
		RegisterContract(s.(skipchain.GetService), dummyKind, verifyDummy)
	}
}

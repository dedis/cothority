package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/dedis/cothority/omniledger/collection"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

var tSuite = suites.MustFind("Ed25519")
var dummyKind = "dummy"
var testInterval = 100 * time.Millisecond

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateSkipchain(t *testing.T) {
	s := newSer(t, 0, testInterval)
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
	genesisMsg.BlockInterval = 100 * time.Millisecond
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
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	// wrong version
	akvresp, err := s.service().AddTransaction(&AddTxRequest{
		Version: CurrentVersion + 1,
	})
	require.NotNil(t, err)

	// missing skipchain
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
		Version: CurrentVersion,
	})
	require.NotNil(t, err)

	// missing transaction
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
	})
	require.NotNil(t, err)

	// the operations below should succeed
	// add the first tx
	tx1, err := createOneClientTx(s.darc.GetBaseID(), dummyKind, s.value, s.signer)
	require.Nil(t, err)
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
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
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
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
				time.Sleep(2 * s.interval)
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
	s := newSer(t, 2, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	serKey := s.tx.Instructions[0].ObjectID.Slice()

	var rep *GetProofResponse
	var i int
	for i = 0; i < 10; i++ {
		time.Sleep(2 * s.interval)
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
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	for i := range s.hosts {
		RegisterContract(s.hosts[i], "panic", panicContractFunc)
		RegisterContract(s.hosts[i], "invalid", invalidContractFunc)
	}

	// tx0 uses the panicing contract, so it should _not_ be stored.
	value1 := []byte("a")
	tx0, err := createOneClientTx(s.darc.GetBaseID(), "panic", value1, s.signer)
	require.Nil(t, err)
	akvresp, err := s.service().AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: tx0,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// tx1 uses the invalid contract, so it should _not_ be stored.
	tx1, err := createOneClientTx(s.darc.GetBaseID(), "invalid", value1, s.signer)
	require.Nil(t, err)
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
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
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: tx2,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	time.Sleep(8 * s.interval)

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

func TestService_LoadBlockInterval(t *testing.T) {
	interval := 200 * time.Millisecond
	s := newSer(t, 1, interval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	dur, err := s.service().LoadBlockInterval(s.sb.SkipChainID())
	require.Nil(t, err)
	require.Equal(t, dur, interval)
}

func TestService_StateChange(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	var latest int64
	f := func(cdb collection.Collection, tx Instruction, c []Coin) ([]StateChange, []Coin, error) {
		cid, _, err := tx.GetContractState(cdb)
		if err != nil {
			return nil, nil, err
		}

		rec, err := cdb.Get(tx.ObjectID.Slice()).Record()
		if err != nil {
			return nil, nil, err
		}

		// create the object if it doesn't exist
		if !rec.Match() {
			if tx.Spawn == nil {
				return nil, nil, errors.New("expected spawn")
			}
			zeroBuf := make([]byte, 8)
			binary.PutVarint(zeroBuf, 0)
			return []StateChange{
				StateChange{
					StateAction: Create,
					ObjectID:    tx.ObjectID.Slice(),
					ContractID:  []byte(cid),
					Value:       zeroBuf,
				},
			}, nil, nil
		}

		if tx.Invoke == nil {
			return nil, nil, errors.New("expected invoke")
		}

		// increment the object value
		vals, err := rec.Values()
		if err != nil {
			return nil, nil, err
		}
		v, _ := binary.Varint(vals[0].([]byte))
		v++

		// we read v back to check later in the test
		latest = v

		vBuf := make([]byte, 8)
		binary.PutVarint(vBuf, v)
		return []StateChange{
			StateChange{
				StateAction: Update,
				ObjectID:    tx.ObjectID.Slice(),
				ContractID:  []byte(cid),
				Value:       vBuf,
			},
		}, nil, nil

	}
	RegisterContract(s.hosts[0], "add", f)

	cdb := s.service().getCollection(s.sb.SkipChainID())
	require.NotNil(t, cdb)

	n := 5
	inst := GenNonce()
	nonce := GenNonce()
	instrs := make([]Instruction, n)
	for i := range instrs {
		instrs[i] = Instruction{
			ObjectID: ObjectID{
				DarcID:     s.darc.GetBaseID(),
				InstanceID: inst,
			},
			Nonce:  nonce,
			Index:  i,
			Length: n,
		}
		if i == 0 {
			instrs[i].Spawn = &Spawn{
				ContractID: "add",
			}
		} else {
			instrs[i].Invoke = &Invoke{}
		}
	}

	cts := []ClientTransaction{
		ClientTransaction{
			Instructions: instrs,
		},
	}

	_, ctsOK, scs, err := s.service().createStateChanges(cdb.coll, cts)
	require.Nil(t, err)
	require.Equal(t, 1, len(ctsOK))
	require.Equal(t, n, len(scs))
	require.Equal(t, latest, int64(n-1))
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
	interval time.Duration
}

func (s *ser) service() *Service {
	return s.services[0]
}

func newSer(t *testing.T, step int, interval time.Duration) *ser {
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

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster, []string{"Spawn_dummy", "Spawn_invalid", "Spawn_panic"}, s.signer.Identity())
	require.Nil(t, err)
	s.darc = &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = interval
	s.interval = genesisMsg.BlockInterval

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
			_, err = s.service().AddTransaction(&AddTxRequest{
				Version:     CurrentVersion,
				SkipchainID: s.sb.SkipChainID(),
				Transaction: tx,
			})
			require.Nil(t, err)
			time.Sleep(4 * s.interval)
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

func invalidContractFunc(cdb collection.Collection, tx Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return nil, nil, errors.New("this invalid contract always returns an error")
}

func panicContractFunc(cdb collection.Collection, tx Instruction, c []Coin) ([]StateChange, []Coin, error) {
	panic("this contract panics")
}

func dummyContractFunc(cdb collection.Collection, tx Instruction, c []Coin) ([]StateChange, []Coin, error) {
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
		RegisterContract(s.(skipchain.GetService), dummyKind, dummyContractFunc)
	}
}

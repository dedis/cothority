package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster, []string{"spawn:dummy"}, signer.Identity())
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

func TestService_AddTransaction(t *testing.T) {
	testAddTransaction(t, 0)
}

func TestService_AddTransactionToFollower(t *testing.T) {
	testAddTransaction(t, 1)
}

func testAddTransaction(t *testing.T, sendToIdx int) {
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
	akvresp, err = s.services[sendToIdx].AddTransaction(&AddTxRequest{
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
					Key:     tx.Instructions[0].InstanceID.Slice(),
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

	serKey := s.tx.Instructions[0].InstanceID.Slice()

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
		Key:     tx1.Instructions[0].InstanceID.Slice(),
	})
	require.Nil(t, err)
	match := pr.Proof.InclusionProof.Match()
	require.False(t, match)

	// Check that tx2 is stored.
	pr, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     tx2.Instructions[0].InstanceID.Slice(),
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
	f := func(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
		cid, _, err := inst.GetContractState(cdb)
		if err != nil {
			return nil, nil, err
		}

		rec, err := cdb.Get(inst.InstanceID.Slice()).Record()
		if err != nil {
			return nil, nil, err
		}

		// create the object if it doesn't exist
		if !rec.Match() {
			if inst.Spawn == nil {
				return nil, nil, errors.New("expected spawn")
			}
			zeroBuf := make([]byte, 8)
			binary.PutVarint(zeroBuf, 0)
			return []StateChange{
				StateChange{
					StateAction: Create,
					InstanceID:  inst.InstanceID.Slice(),
					ContractID:  []byte(cid),
					Value:       zeroBuf,
				},
			}, nil, nil
		}

		if inst.Invoke == nil {
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
				InstanceID:  inst.InstanceID.Slice(),
				ContractID:  []byte(cid),
				Value:       vBuf,
			},
		}, nil, nil

	}
	RegisterContract(s.hosts[0], "add", f)

	cdb := s.service().getCollection(s.sb.SkipChainID())
	require.NotNil(t, cdb)

	n := 5
	inst := genSubID()
	nonce := GenNonce()
	instrs := make([]Instruction, n)
	for i := range instrs {
		instrs[i] = Instruction{
			InstanceID: InstanceID{
				DarcID: s.darc.GetBaseID(),
				SubID:  inst,
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

func TestService_DarcEvolutionFail(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	d2 := s.darc.Copy()
	require.Nil(t, d2.EvolveFrom(s.darc))

	// first we create a bad request, i.e., with an invalid version number
	d2.Version = 11
	pr := s.testDarcEvolution(t, *d2, true)

	// parse the darc
	require.True(t, pr.InclusionProof.Match())
	_, vs, err := pr.KeyValue()
	require.Nil(t, err)
	d22, err := darc.NewFromProtobuf(vs[0])
	require.Nil(t, err)
	require.False(t, d22.Equal(d2))
	require.True(t, d22.Equal(s.darc))
}

func TestService_DarcEvolution(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	d2 := s.darc.Copy()
	require.Nil(t, d2.EvolveFrom(s.darc))
	pr := s.testDarcEvolution(t, *d2, false)

	// parse the darc
	require.True(t, pr.InclusionProof.Match())
	_, vs, err := pr.KeyValue()
	require.Nil(t, err)
	d22, err := darc.NewFromProtobuf(vs[0])
	require.Nil(t, err)
	require.True(t, d22.Equal(d2))
}

func TestService_DarcSpawn(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	id := []darc.Identity{s.signer.Identity()}
	darc2 := darc.NewDarc(darc.InitRulesWith(id, id, invokeEvolve),
		[]byte("next darc"))
	darc2.Rules.AddRule("spawn:rain", darc2.Rules.GetSignExpr())
	darc2Buf, err := darc2.ToProto()
	require.Nil(t, err)
	darc2Copy, err := darc.NewFromProtobuf(darc2Buf)
	require.Nil(t, err)
	require.True(t, darc2.Equal(darc2Copy))

	ctx := ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: InstanceID{
				DarcID: s.darc.GetBaseID(),
				SubID:  SubID{},
			},
			Nonce:  GenNonce(),
			Index:  0,
			Length: 1,
			Spawn: &Spawn{
				ContractID: ContractDarcID,
				Args: []Argument{{
					Name:  "darc",
					Value: darc2Buf,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(s.signer))

	s.sendTx(t, ctx)
	pr := s.waitProof(t, InstanceID{darc2.GetBaseID(), SubID{}})
	require.True(t, pr.InclusionProof.Match())
}

func TestService_SetLeader(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	defer closeQueues(s.local)

	for _, service := range s.services {
		// everyone should have the same leader after the genesis block is stored
		leader := service.leaderMap.get(string(s.sb.SkipChainID()))
		require.NotNil(t, leader)
		require.True(t, leader.Equal(s.services[0].ServerIdentity()))
	}
}

func darcToTx(t *testing.T, d2 darc.Darc, signer darc.Signer) ClientTransaction {
	d2Buf, err := d2.ToProto()
	require.Nil(t, err)
	invoke := Invoke{
		Command: "evolve",
		Args: []Argument{
			Argument{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := Instruction{
		InstanceID: InstanceID{
			DarcID: d2.GetBaseID(),
			SubID:  SubID{},
		},
		Nonce:  GenNonce(),
		Index:  0,
		Length: 1,
		Invoke: &invoke,
	}
	require.Nil(t, instr.SignBy(signer))
	return ClientTransaction{
		Instructions: []Instruction{instr},
	}
}

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
	sb       *skipchain.SkipBlock
	value    []byte
	darc     *darc.Darc
	signer   darc.Signer
	tx       ClientTransaction
	interval time.Duration
}

func (s *ser) service() *Service {
	return s.services[0]
}

func (s *ser) waitProof(t *testing.T, id InstanceID) Proof {
	var pr Proof
	for i := 0; i < 10; i++ {
		// try to get the darc back, we should get the genesis back instead
		resp, err := s.service().GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     id.Slice(),
			ID:      s.sb.SkipChainID(),
		})
		require.Nil(t, err)
		pr = resp.Proof
		if pr.InclusionProof.Match() {
			break
		}

		// wait for the block to be processed
		time.Sleep(s.interval)
	}

	return pr
}

func (s *ser) sendTx(t *testing.T, ctx ClientTransaction) {
	s.sendTxTo(t, ctx, 0)
}

func (s *ser) sendTxTo(t *testing.T, ctx ClientTransaction, idx int) {
	_, err := s.services[idx].AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.sb.SkipChainID(),
		Transaction: ctx,
	})
	require.Nil(t, err)
}

// caller gives us a darc, and we try to make an evolution request.
func (s *ser) testDarcEvolution(t *testing.T, d2 darc.Darc, fail bool) (pr *Proof) {
	ctx := darcToTx(t, d2, s.signer)
	s.sendTx(t, ctx)
	for i := 0; i < 10; i++ {
		resp, err := s.service().GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     InstanceID{d2.GetBaseID(), SubID{}}.Slice(),
			ID:      s.sb.SkipChainID(),
		})
		require.Nil(t, err)
		pr = &resp.Proof
		vs, err := pr.InclusionProof.Values()
		require.Nil(t, err)
		d, err := darc.NewFromProtobuf(vs[0].([]byte))
		require.Nil(t, err)
		if d.Equal(&d2) {
			return
		}
		time.Sleep(s.interval)
	}
	if !fail {
		t.Fatal("couldn't store new darc")
	}
	return
}

func newSer(t *testing.T, step int, interval time.Duration) *ser {
	s := &ser{
		local:  onet.NewTCPTest(tSuite),
		value:  []byte("anyvalue"),
		signer: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(2, true)

	for _, sv := range s.local.GetServices(s.hosts, OmniledgerID) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}
	registerDummy(s.hosts)

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster,
		[]string{"spawn:dummy", "spawn:invalid", "spawn:panic", "spawn:darc"}, s.signer.Identity())
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
		services := local.GetServices([]*onet.Server{server}, OmniledgerID)
		services[0].(*Service).ClosePolling()
	}
}

func invalidContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return nil, nil, errors.New("this invalid contract always returns an error")
}

func panicContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	panic("this contract panics")
}

func dummyContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	args := inst.Spawn.Args[0].Value
	cid, _, err := inst.GetContractState(cdb)
	if err != nil {
		return nil, nil, err
	}
	return []StateChange{
		NewStateChange(Create, inst.InstanceID, cid, args),
	}, nil, nil
}

func registerDummy(servers []*onet.Server) {
	// For testing - there must be a better way to do that. But putting
	// services []skipchain.GetService in the method signature doesn't work :(
	for _, s := range servers {
		RegisterContract(s, dummyKind, dummyContractFunc)
	}
}

func genSubID() (n SubID) {
	random.Bytes(n[:], random.New())
	return n
}

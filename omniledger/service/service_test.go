package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/darc/expression"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")
var dummyKind = "dummy"
var slowKind = "slow"
var invalidKind = "invalid"
var testInterval = 200 * time.Millisecond

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateSkipchain(t *testing.T) {
	s := newSer(t, 0, testInterval)
	defer s.local.CloseAll()

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
	testAddTransaction(t, 0, false)
}

func TestService_AddTransaction_ToFollower(t *testing.T) {
	testAddTransaction(t, 1, false)
}

func TestService_AddTransaction_WithFailure(t *testing.T) {
	testAddTransaction(t, 0, true)
}

func testAddTransaction(t *testing.T, sendToIdx int, failure bool) {
	var s *ser
	if failure {
		s = newSerN(t, 1, time.Second, 4, false)
		for _, service := range s.services {
			service.SetPropagationTimeout(time.Second)
		}
	} else {
		s = newSer(t, 1, testInterval)
	}
	defer s.local.CloseAll()

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

	if failure {
		// kill a child conode and adding tx should still succeed
		s.hosts[len(s.hosts)-1].Pause()
		defer s.hosts[len(s.hosts)-1].Unpause()
	}

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
			require.NoError(t, s.service().tryLoad())
		}
		for _, tx := range txs {
			newId := tx.Instructions[0].Hash()
			ok := false
			for ct := 0; ct < 10; ct++ {
				time.Sleep(2 * s.interval)
				pr, err := s.service().GetProof(&GetProof{
					Version: CurrentVersion,
					ID:      s.sb.SkipChainID(),
					Key:     newId,
				})
				if err != nil {
					log.Error(err)
					continue
				}
				require.Equal(t, CurrentVersion, pr.Version)
				if pr.Proof.InclusionProof.Match() {
					require.Nil(t, pr.Proof.Verify(s.sb.SkipChainID()))
					_, vs, err := pr.Proof.KeyValue()
					require.Nil(t, err)
					require.True(t, bytes.Equal(tx.Instructions[0].Spawn.Args[0].Value, vs[0]))
					ok = true
					break
				}
			}
			require.True(t, ok)
		}
	}
}

func TestService_GetProof(t *testing.T) {
	s := newSer(t, 2, testInterval)
	defer s.local.CloseAll()

	serKey := s.tx.Instructions[0].Hash()

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

// TODO: Fix issue #1379.
//
// This test currently fails because of:
// E : (       service.(*Service).verifyClientTx: 277) - local://127.0.0.1:2000 instr #1, darc not found: no match found
// E : (   service.(*Service).verifyAndFilterTxs: 266) - local://127.0.0.1:2000 tx #0, darc not found: no match found
//
// This is because inter-instruction dependencies are not correctly handled by verifyAndFilterTxs,
// which is operating on the live database to find the darcID, so it cannot find the darcID associated
// with the instance created in the first instruction while trying to check the second instruction.
func TestService_Depending(t *testing.T) {
	t.Skip("Issue #1379.")

	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Create a client tx with two instructions in it where the second one depends on
	// the first one having executed.

	// First instruction: spawn a dummy value.
	in1, err := createInstr(s.darc.GetBaseID(), dummyKind, []byte("something to delete"), s.signer)
	require.NoError(t, err)

	// Set the length to reflect there are two.
	// Need to sign it again because we hacked on it.
	in1.Length = 2
	in1.SignBy(s.darc.GetBaseID(), s.signer)

	// Second instruction: delete the value we just spawned.
	in2 := Instruction{
		InstanceID: InstanceIDFromSlice(in1.Hash()),
		Delete:     &Delete{},
		Nonce:      GenNonce(),
		Index:      1,
		Length:     2,
	}
	in2.SignBy(s.darc.GetBaseID(), s.signer)

	tx := ClientTransaction{
		Instructions: []Instruction{in1, in2},
	}

	_, err = s.services[0].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.sb.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 2,
	})
	require.Nil(t, err)
}

func TestService_WaitInclusion(t *testing.T) {
	for i := 0; i < 3; i++ {
		log.Lvl1("Testing inclusion when sending to service", i)
		waitInclusion(t, i)
	}
}

func waitInclusion(t *testing.T, client int) {
	s := newSer(t, 2, testInterval)
	defer s.local.CloseAll()

	// Create a transaction without waiting
	log.Lvl1("Create transaction and don't wait")
	pr := sendTransaction(t, s, client, dummyKind, 0)
	require.False(t, pr.InclusionProof.Match())

	// Create a transaction and wait
	log.Lvl1("Create correct transaction and wait")
	pr = sendTransaction(t, s, client, dummyKind, 10)
	require.True(t, pr.InclusionProof.Match())

	// Create a failing transaction and wait
	done := make(chan bool)
	go func() {
		log.Lvl1("Create wrong transaction and wait")
		pr := sendTransaction(t, s, client, invalidKind, 10)
		require.False(t, pr.InclusionProof.Match())
		done <- true
	}()

	// Wait two intervals before sending a new transaction
	time.Sleep(2 * s.interval)

	// Create a transaction and wait
	log.Lvl1("Create second correct transaction and wait")
	pr = sendTransaction(t, s, client, dummyKind, 10)
	require.True(t, pr.InclusionProof.Match())
	select {
	case <-done:
		require.Fail(t, "go-routine should not be done yet")
	default:
	}

	<-done
}

// Sends too many transactions to the ledger and waits for all blocks to be done.
func TestService_FloodLedger(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Store the latest block
	reply, err := skipchain.NewClient().GetUpdateChain(s.sb.Roster, s.sb.SkipChainID())
	require.Nil(t, err)
	before := reply.Update[len(reply.Update)-1]

	log.Lvl1("Create 10 transactions and don't wait")
	for i := 0; i < 10; i++ {
		sendTransaction(t, s, 0, slowKind, 0)
	}
	// Send a last transaction and wait for it to be included
	sendTransaction(t, s, 0, dummyKind, 100)

	// Suppose we need at least 2 blocks (slowKind waits 1/5 interval for each execution)
	reply, err = skipchain.NewClient().GetUpdateChain(s.sb.Roster, s.sb.SkipChainID())
	require.Nil(t, err)
	latest := reply.Update[len(reply.Update)-1]
	if latest.Index-before.Index < 2 {
		t.Fatalf("didn't get at least 2 blocks: index before %d, index after %v", before.Index, latest.Index)
	}
}

func sendTransaction(t *testing.T, s *ser, client int, kind string, wait int) Proof {
	tx, err := createOneClientTx(s.darc.GetBaseID(), kind, s.value, s.signer)
	require.Nil(t, err)
	ser := s.services[client]
	_, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.sb.SkipChainID(),
		Transaction:   tx,
		InclusionWait: wait,
	})
	switch kind {
	case dummyKind:
		require.Nil(t, err)
	case slowKind:
		require.Nil(t, err)
	case invalidKind:
		require.NotNil(t, err)
	}
	// The instruction should not be included (except if we're very unlucky)
	rep, err := ser.GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     tx.Instructions[0].Hash(),
	})
	require.Nil(t, err)
	return rep.Proof
}

func TestService_InvalidVerification(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	for i := range s.hosts {
		RegisterContract(s.hosts[i], "panic", panicContractFunc)
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
	tx1, err := createOneClientTx(s.darc.GetBaseID(), invalidKind, value1, s.signer)
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
		Key:     tx1.Instructions[0].Hash(),
	})
	require.Nil(t, err)
	match := pr.Proof.InclusionProof.Match()
	require.False(t, match)

	// Check that tx2 is stored.
	pr, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     tx2.Instructions[0].Hash(),
	})
	require.Nil(t, err)
	match = pr.Proof.InclusionProof.Match()
	require.True(t, match)
}

func TestService_LoadBlockInterval(t *testing.T) {
	interval := 200 * time.Millisecond
	s := newSer(t, 1, interval)
	defer s.local.CloseAll()

	dur, err := s.service().LoadBlockInterval(s.sb.SkipChainID())
	require.Nil(t, err)
	require.Equal(t, dur, interval)
}

func TestService_StateChange(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

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
	inst := genID()
	nonce := GenNonce()
	instrs := make([]Instruction, n)
	for i := range instrs {
		instrs[i] = Instruction{
			InstanceID: inst,
			Nonce:      nonce,
			Index:      i,
			Length:     n,
		}
		if i == 0 {
			instrs[i].Spawn = &Spawn{
				ContractID: "add",
			}
		} else {
			instrs[i].Invoke = &Invoke{}
		}
	}

	instrs2 := make([]Instruction, 1)
	instrs2[0] = Instruction{
		InstanceID: inst,
		Nonce:      nonce,
		Index:      0,
		Length:     1,
		Spawn: &Spawn{
			ContractID: "not-add",
		},
	}

	cts := []ClientTransaction{
		ClientTransaction{
			Instructions: instrs,
		},
		ClientTransaction{
			Instructions: instrs2,
		},
	}

	_, ctsOK, ctsBad, scs, err := s.service().createStateChanges(cdb.coll, s.sb.SkipChainID(), cts, noTimeout)
	require.Nil(t, err)
	require.Equal(t, 1, len(ctsOK))
	require.Equal(t, 1, len(ctsBad))
	require.Equal(t, n, len(scs))
	require.Equal(t, latest, int64(n-1))
}

func TestService_DarcEvolutionFail(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

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
			InstanceID: InstanceIDFromSlice(s.darc.GetBaseID()),
			Nonce:      GenNonce(),
			Index:      0,
			Length:     1,
			Spawn: &Spawn{
				ContractID: ContractDarcID,
				Args: []Argument{{
					Name:  "darc",
					Value: darc2Buf,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(s.darc.GetBaseID(), s.signer))

	s.sendTx(t, ctx)
	pr := s.waitProof(t, InstanceIDFromSlice(darc2.GetBaseID()))
	require.True(t, pr.InclusionProof.Match())
}

func TestService_DarcDelegation(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Spawn second darc with a new owner/signer, but delegate its spawn
	// rule to the first darc
	signer2 := darc.NewSignerEd25519(nil, nil)
	id2 := []darc.Identity{signer2.Identity()}
	darc2 := darc.NewDarc(darc.InitRules(id2, id2),
		[]byte("second darc"))
	darc2.Rules.AddRule("spawn:darc", expression.InitOrExpr(s.darc.GetIdentityString()))
	darc2Buf, err := darc2.ToProto()
	require.Nil(t, err)
	darc2Copy, err := darc.NewFromProtobuf(darc2Buf)
	require.Nil(t, err)
	require.True(t, darc2.Equal(darc2Copy))
	ctx := ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: InstanceIDFromSlice(s.darc.GetBaseID()),
			Nonce:      GenNonce(),
			Index:      0,
			Length:     1,
			Spawn: &Spawn{
				ContractID: ContractDarcID,
				Args: []Argument{{
					Name:  "darc",
					Value: darc2Buf,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(s.darc.GetBaseID(), s.signer))
	s.sendTx(t, ctx)
	pr := s.waitProof(t, InstanceIDFromSlice(darc2.GetBaseID()))
	require.True(t, pr.InclusionProof.Match())

	// Spawn third darc from the second one, but sign the request with
	// the signer of the first darc to test delegation
	signer3 := darc.NewSignerEd25519(nil, nil)
	id3 := []darc.Identity{signer3.Identity()}
	darc3 := darc.NewDarc(darc.InitRules(id3, id3),
		[]byte("third darc"))
	darc3Buf, err := darc3.ToProto()
	require.Nil(t, err)
	darc3Copy, err := darc.NewFromProtobuf(darc3Buf)
	require.Nil(t, err)
	require.True(t, darc3.Equal(darc3Copy))
	ctx = ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: InstanceIDFromSlice(darc2.GetBaseID()),
			Nonce:      GenNonce(),
			Index:      0,
			Length:     1,
			Spawn: &Spawn{
				ContractID: ContractDarcID,
				Args: []Argument{{
					Name:  "darc",
					Value: darc3Buf,
				}},
			},
		}},
	}

	require.Nil(t, ctx.Instructions[0].SignBy(darc2.GetBaseID(), s.signer))
	s.sendTx(t, ctx)
	pr = s.waitProof(t, InstanceIDFromSlice(darc3.GetBaseID()))
	require.True(t, pr.InclusionProof.Match())
}

func TestService_GetLeader(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	for _, service := range s.services {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(s.sb.SkipChainID())
		require.NoError(t, err)
		require.NotNil(t, leader)
		require.True(t, leader.Equal(s.services[0].ServerIdentity()))
	}
}

func TestService_SetConfig(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	ctx, newConfig := createConfigTx(t, s, true)
	s.sendTx(t, ctx)

	// wait for a change
	for i := 0; i < 5; i++ {
		time.Sleep(s.interval)
		config, err := s.service().LoadConfig(s.sb.SkipChainID())
		require.NoError(t, err)

		if config.BlockInterval == newConfig.BlockInterval {
			return
		}
	}
	require.Fail(t, "did not find new config in time")
}

func TestService_SetBadConfig(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	ctx, badConfig := createConfigTx(t, s, false)
	s.sendTx(t, ctx)

	// wait for a change, which should not happend
	for i := 0; i < 5; i++ {
		time.Sleep(s.interval)
		config, err := s.service().LoadConfig(s.sb.SkipChainID())
		require.NoError(t, err)

		if badConfig.Roster.List[0].Equal(config.Roster.List[0]) {
			require.Fail(t, "found a bad config")
		}
	}
}

// TestService_RotateLeader is an end-to-end test for view-change. We kill the
// current leader, at index 0. Then the node at index 1 becomes the new leader.
// Then, we try to send a transaction to a follower, at index 2. The new leader
// should poll for new transactions and eventually make a new block containing
// that transaction. The new transaction should be stored on all followers.
func TestService_RotateLeader(t *testing.T) {
	interval := 2 * time.Second
	s := newSerN(t, 1, interval, 4, true)
	defer s.local.CloseAll()

	for _, service := range s.services {
		service.SetPropagationTimeout(interval / 2)
	}

	// Stop the leader, then the next node should take over.
	s.service().TestClose()
	s.hosts[0].Pause()

	// wait for the new block
	var ok bool
	for i := 0; i < 5; i++ {
		time.Sleep(2 * s.interval)
		config, err := s.services[1].LoadConfig(s.sb.SkipChainID())
		require.NoError(t, err)

		if config.Roster.List[0].Equal(s.services[1].ServerIdentity()) {
			ok = true
			break
		}
	}
	require.True(t, ok, "leader rotation failed")

	// check that the leader is updated for all nodes
	for _, service := range s.services[1:] {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(s.sb.SkipChainID())
		require.NoError(t, err)
		require.NotNil(t, leader)
		require.True(t, leader.Equal(s.services[1].ServerIdentity()))
	}

	// try to send a transaction to the node on index 2, which is a
	// follower (not the new leader)
	tx1, err := createOneClientTx(s.darc.GetBaseID(), dummyKind, s.value, s.signer)
	require.NoError(t, err)
	s.sendTxTo(t, tx1, 2)

	// wait for the transaction to be stored on the new leader, because it
	// polls for new transactions
	pr := s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID, 1)
	require.True(t, pr.InclusionProof.Match())

	// the transaction should also be stored on followers, at index 2 and 3
	pr = s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID, 2)
	require.True(t, pr.InclusionProof.Match())
	pr = s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID, 3)
	require.True(t, pr.InclusionProof.Match())
}

func TestService_DarcToSc(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	darcID := s.darc.GetBaseID()
	scID := s.sb.SkipChainID()

	// check that the mapping is correct
	for _, service := range s.services {
		require.True(t, service.darcToSc[string(darcID)].Equal(scID))
	}

	// remove the mapping and then load it again
	for _, service := range s.services {
		service.darcToSc = make(map[string]skipchain.SkipBlockID)
		require.NoError(t, service.tryLoad())
	}

	// check that the mapping is still correct
	for _, service := range s.services {
		require.True(t, service.darcToSc[string(darcID)].Equal(scID))
	}
}

func TestService_StateChangeCache(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Register a stateful contract, so we can monitor how many times that
	// the contract gets called. Using the state change cache, we should
	// only call it once.
	contractID := "stateShangeCacheTest"
	var ctr int
	contract := func(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
		ctr++
		return []StateChange{}, []Coin{}, nil
	}
	s.service().registerContract(contractID, contract)

	scID := s.sb.SkipChainID()
	coll := s.service().getCollection(scID).coll
	tx, err := createOneClientTx(s.darc.GetBaseID(), contractID, []byte{}, s.signer)
	txs := ClientTransactions([]ClientTransaction{tx})
	require.NoError(t, err)
	root, ctsOK, ctsBad, states, err := s.service().createStateChanges(coll, scID, txs, noTimeout)
	require.NoError(t, err)
	require.Equal(t, ctr, 1)

	// If we call createStateChanges again, then it should load it from the
	// cache, which means that ctr is still one (we do not call the
	// contract twice).
	root1, ctsOK1, ctsBad1, states1, err := s.service().createStateChanges(coll, scID, txs, noTimeout)
	require.NoError(t, err)
	require.Equal(t, ctr, 1)

	require.Equal(t, root, root1)
	require.Equal(t, ctsOK, ctsOK1)
	require.Equal(t, ctsBad, ctsBad1)
	require.Equal(t, states, states1)

	// If we remove the cache, then we expect the contract to be called
	// again, i.e., ctr == 2.
	s.service().stateChangeCache = newStateChangeCache()
	require.NoError(t, err)
	root2, ctsOK2, ctsBad2, states2, err := s.service().createStateChanges(coll, scID, txs, noTimeout)
	require.NoError(t, err)
	require.Equal(t, root, root2)
	require.Equal(t, ctsOK, ctsOK2)
	require.Equal(t, ctsBad, ctsBad2)
	require.Equal(t, states, states2)
	require.Equal(t, ctr, 2)
}

func createConfigTx(t *testing.T, s *ser, isgood bool) (ClientTransaction, ChainConfig) {
	var config ChainConfig
	if isgood {
		config = ChainConfig{420 * time.Millisecond, *s.roster}
	} else {
		config = ChainConfig{-1, *s.roster.RandomSubset(s.services[1].ServerIdentity(), 2)}
	}
	configBuf, err := protobuf.Encode(&config)
	require.NoError(t, err)

	cfgid := DeriveConfigID(s.darc.GetBaseID())

	ctx := ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: cfgid,
			Nonce:      GenNonce(),
			Index:      0,
			Length:     1,
			Invoke: &Invoke{
				Command: "update_config",
				Args: []Argument{{
					Name:  "config",
					Value: configBuf,
				}},
			},
		}},
	}
	require.NoError(t, ctx.Instructions[0].SignBy(s.darc.GetBaseID(), s.signer))
	return ctx, config
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
		InstanceID: InstanceIDFromSlice(d2.GetBaseID()),
		Nonce:      GenNonce(),
		Index:      0,
		Length:     1,
		Invoke:     &invoke,
	}
	require.Nil(t, instr.SignBy(d2.GetBaseID(), signer))
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
	return s.waitProofWithIdx(t, id, 0)
}

func (s *ser) waitProofWithIdx(t *testing.T, id InstanceID, idx int) Proof {
	var pr Proof
	for i := 0; i < 10; i++ {
		resp, err := s.services[idx].GetProof(&GetProof{
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
			Key:     d2.GetBaseID(),
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
	return newSerN(t, step, interval, 3, false)
}

func newSerN(t *testing.T, step int, interval time.Duration, n int, viewchange bool) *ser {
	s := &ser{
		local:  onet.NewLocalTestT(tSuite, t),
		value:  []byte("anyvalue"),
		signer: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(n, true)

	for _, sv := range s.local.GetServices(s.hosts, OmniledgerID) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}
	registerDummy(s.hosts)

	if viewchange {
		for _, service := range s.services {
			service.EnableViewChange()
		}
	}

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster,
		[]string{"spawn:dummy", "spawn:invalid", "spawn:panic", "spawn:darc", "invoke:update_config", "spawn:slow"}, s.signer.Identity())
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

func invalidContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return nil, nil, errors.New("this invalid contract always returns an error")
}

func panicContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	panic("this contract panics")
}

func dummyContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	cid, _, err := inst.GetContractState(cdb)
	if err != nil {
		return nil, nil, err
	}

	_, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	switch inst.GetType() {
	case SpawnType:
		return []StateChange{
			NewStateChange(Create, InstanceIDFromSlice(inst.Hash()), cid, inst.Spawn.Args[0].Value, darcID),
		}, nil, nil
	case DeleteType:
		return []StateChange{
			NewStateChange(Remove, inst.InstanceID, cid, nil, darcID),
		}, nil, nil
	default:
		panic("should not get here")
	}
}

func slowContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	// This has to sleep for less than testInterval / 2 or else it will
	// block the system from processing txs. See #1359.
	time.Sleep(testInterval / 5)
	return dummyContractFunc(cdb, inst, c)
}

func registerDummy(servers []*onet.Server) {
	// For testing - there must be a better way to do that. But putting
	// services []skipchain.GetService in the method signature doesn't work :(
	for _, s := range servers {
		RegisterContract(s, dummyKind, dummyContractFunc)
		RegisterContract(s, slowKind, slowContractFunc)
		RegisterContract(s, invalidKind, invalidContractFunc)
	}
}

func genID() (i InstanceID) {
	random.Bytes(i[:], random.New())
	return i
}

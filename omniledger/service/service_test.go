package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/darc/expression"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")
var dummyContract = "dummy"
var slowContract = "slow"
var giantContract = "giant"
var invalidContract = "invalid"
var testInterval = 500 * time.Millisecond

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateGenesisBlock(t *testing.T) {
	s := newSer(t, 0, testInterval)
	defer s.local.CloseAll()

	// invalid version, missing transaction
	resp, err := s.service().CreateGenesisBlock(&CreateGenesisBlock{
		Version: 0,
		Roster:  *s.roster,
	})
	require.NotNil(t, err)

	// invalid: max block too small, big
	resp, err = s.service().CreateGenesisBlock(&CreateGenesisBlock{
		Version:      0,
		Roster:       *s.roster,
		MaxBlockSize: 3000,
	})
	require.NotNil(t, err)
	resp, err = s.service().CreateGenesisBlock(&CreateGenesisBlock{
		Version:      0,
		Roster:       *s.roster,
		MaxBlockSize: 30 * 1e6,
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
	require.Nil(t, err)
	genesisMsg.BlockInterval = 100 * time.Millisecond
	genesisMsg.MaxBlockSize = 1 * 1e6

	// finally passing
	resp, err = s.service().CreateGenesisBlock(genesisMsg)
	require.Nil(t, err)
	assert.Equal(t, CurrentVersion, resp.Version)
	assert.NotNil(t, resp.Skipblock)

	proof, err := s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		Key:     genesisMsg.GenesisDarc.GetID(),
		ID:      resp.Skipblock.SkipChainID(),
	})
	require.Nil(t, err)
	require.Nil(t, proof.Proof.Verify(resp.Skipblock.SkipChainID()))
	k, _, err := proof.Proof.KeyValue()
	require.Nil(t, err)
	require.EqualValues(t, genesisMsg.GenesisDarc.GetID(), k)

	interval, maxsz, err := s.service().LoadBlockInfo(resp.Skipblock.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, interval, genesisMsg.BlockInterval)
	require.Equal(t, maxsz, genesisMsg.MaxBlockSize)
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
	if testing.Short() {
		t.Skip("test takes too long for Travis")
	}
	testAddTransaction(t, 0, true)
}

func TestService_AddTransaction_WithFailure_OnFollower(t *testing.T) {
	testAddTransaction(t, 1, true)
}

func testAddTransaction(t *testing.T, sendToIdx int, failure bool) {
	var s *ser
	if failure {
		s = newSerN(t, 1, 500*time.Millisecond, 4, false)
		for _, service := range s.services {
			service.SetPropagationTimeout(4 * time.Second)
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
		log.Lvl1("Pausing (killing) conode", s.hosts[len(s.hosts)-1].Address())
		s.services[len(s.hosts)-1].TestClose()
		s.hosts[len(s.hosts)-1].Pause()
	}

	// the operations below should succeed
	// add the first tx
	log.Lvl1("adding the first tx")
	tx1, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
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
	log.Lvl1("adding the second tx")
	value2 := []byte("value2")
	tx2, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, value2, s.signer)
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
	log.Lvl1("reading the transactions back")
	txs := []ClientTransaction{tx1, tx2}
	for i := 0; i < 2; i++ {
		if i == 1 {
			// Now read the key/values from a new service
			log.Lvl1("Recreate services and fetch keys again")
			require.NoError(t, s.service().startAllChains())
		}
		for _, tx := range txs {
			pr := s.waitProofWithIdx(t, tx.Instructions[0].Hash(), 0)
			require.Nil(t, pr.Verify(s.sb.SkipChainID()))
			_, vs, err := pr.KeyValue()
			require.Nil(t, err)
			require.True(t, bytes.Equal(tx.Instructions[0].Spawn.Args[0].Value, vs[0]))

			// check that the database has this new block's index recorded
			require.Equal(t, pr.Latest.Index, s.services[0].getCollection(pr.Latest.SkipChainID()).getIndex())
		}
	}

	// Bring the failed node back up and it should also see the transactions.
	if failure {
		log.Lvl1("bringing the failed node back up")
		s.services[len(s.hosts)-1].closed = false
		s.hosts[len(s.hosts)-1].Unpause()
		require.NoError(t, s.services[len(s.hosts)-1].startAllChains())

		time.Sleep(s.interval)
		for _, tx := range txs {
			pr := s.waitProofWithIdx(t, tx.Instructions[0].Hash(), len(s.hosts)-1)
			require.Nil(t, pr.Verify(s.sb.SkipChainID()))
			_, vs, err := pr.KeyValue()
			require.Nil(t, err)
			require.True(t, bytes.Equal(tx.Instructions[0].Spawn.Args[0].Value, vs[0]))
			// check that the database has this new block's index recorded
			require.Equal(t, pr.Latest.Index, s.services[len(s.hosts)-1].getCollection(pr.Latest.SkipChainID()).getIndex())
		}

		// Try to add a new transaction to the node that failed (but is
		// now running) and it should work.
		log.Lvl1("making a last transaction")
		pr, err, err2 := sendTransaction(t, s, len(s.hosts)-1, dummyContract, 10)
		require.NoError(t, err)
		require.NoError(t, err2)
		require.True(t, pr.InclusionProof.Match())
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

// Test that inter-instruction dependencies are correctly handled.
func TestService_Depending(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Create a client tx with two instructions in it where the second one depends on
	// the first one having executed.

	// First instruction: spawn a dummy value.
	in1, err := createInstr(s.darc.GetBaseID(), dummyContract, []byte("something to delete"), s.signer)
	require.NoError(t, err)

	// Set the length to reflect there are two.
	// Need to sign it again because we hacked on it.
	in1.Length = 2
	in1.SignBy(s.darc.GetBaseID(), s.signer)

	// Second instruction: delete the value we just spawned.
	in2 := Instruction{
		InstanceID: NewInstanceID(in1.Hash()),
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

	cdb := s.service().getCollection(s.sb.SkipChainID())
	_, _, _, err = cdb.GetValues(in1.Hash())
	require.NotNil(t, err)
	require.Equal(t, errKeyNotSet, err)

	// We need to wait a bit for the propagation to finish because the
	// skipchain service might decide to update forward links by adding
	// additional blocks. How do we make sure that the test closes only
	// after the forward links are all updated?
	time.Sleep(time.Second)
}

func TestService_LateBlock(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Hook the verifier in order delay the arrival and test timestamp checking.
	ser := s.services[0]
	c := ser.Context
	skipchain.RegisterVerification(c, verifyOmniLedger, func(newID []byte, newSB *skipchain.SkipBlock) bool {
		// Make this block arrive late compared to it's timestamp. The window will be
		// 1000ms, so sleep 100 more.
		time.Sleep(2100 * time.Millisecond)
		return ser.verifySkipBlock(newID, newSB)
	})

	tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.Nil(t, err)
	_, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.sb.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 5,
	})
	require.Error(t, err)
}

func TestService_BadDataHeader(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	ser := s.services[0]
	c := ser.Context
	skipchain.RegisterVerification(c, verifyOmniLedger, func(newID []byte, newSB *skipchain.SkipBlock) bool {
		// Hack up the DataHeader to make the CollectionRoot the wrong size.
		var header DataHeader
		err := protobuf.DecodeWithConstructors(newSB.Data, &header, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			t.Fatal(err)
		}
		header.CollectionRoot = append(header.CollectionRoot, 0xff)
		newSB.Data, _ = protobuf.Encode(header)

		return ser.verifySkipBlock(newID, newSB)
	})

	tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.Nil(t, err)
	_, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.sb.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 5,
	})
	require.Error(t, err)
}

func txResultsFromBlock(sb *skipchain.SkipBlock) (TxResults, error) {
	var body DataBody
	err := protobuf.DecodeWithConstructors(sb.Payload, &body, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}
	return body.TxResults, nil
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
	pr, err, err2 := sendTransaction(t, s, client, dummyContract, 0)
	require.NoError(t, err)
	require.NoError(t, err2)
	require.False(t, pr.InclusionProof.Match())

	log.Lvl1("Create correct transaction and wait")
	pr, err, err2 = sendTransaction(t, s, client, dummyContract, 10)
	require.NoError(t, err)
	require.NoError(t, err2)
	require.True(t, pr.InclusionProof.Match())

	// We expect to see both transactions in the block in pr.
	txr, err := txResultsFromBlock(&pr.Latest)
	require.NoError(t, err)
	require.Equal(t, len(txr), 2)

	log.Lvl1("Create wrong transaction and wait")
	pr, err, err2 = sendTransaction(t, s, client, invalidContract, 10)
	require.Contains(t, err.Error(), "transaction is in block, but got refused")
	require.NoError(t, err2)

	// We expect to see only the refused transaction in the block in pr.
	require.True(t, len(pr.Latest.Payload) > 0)
	txr, err = txResultsFromBlock(&pr.Latest)
	require.NoError(t, err)
	require.Equal(t, len(txr), 1)
	require.False(t, txr[0].Accepted)

	log.Lvl1("Create wrong transaction, no wait")
	sendTransaction(t, s, client, invalidContract, 0)
	log.Lvl1("Create second correct transaction and wait")
	pr, err, err2 = sendTransaction(t, s, client, dummyContract, 10)
	require.NoError(t, err)
	require.NoError(t, err2)
	require.True(t, pr.InclusionProof.Match())

	// We expect to see the refused transaction and the good one in the block in pr.
	txr, err = txResultsFromBlock(&pr.Latest)
	require.NoError(t, err)

	if len(txr) == 1 {
		log.Lvl1("the good tx ended up in it's own block")
		require.True(t, txr[0].Accepted)

		// Look in the previous block for the failed one.
		prev := s.service().db().GetByID(pr.Latest.BackLinkIDs[0])
		require.NotNil(t, prev)
		txr, err = txResultsFromBlock(prev)
		require.NoError(t, err)
		require.Equal(t, len(txr), 1)
		require.False(t, txr[0].Accepted)
	} else {
		log.Lvl1("they are both in this block")
		require.False(t, txr[0].Accepted)
		require.True(t, txr[1].Accepted)
	}

	// We need to wait a bit for the propagation to finish because the
	// skipchain service might decide to update forward links by adding
	// additional blocks. How do we make sure that the test closes only
	// after the forward links are all updated?
	time.Sleep(time.Second)
}

// Sends too many transactions to the ledger and waits for all blocks to be done.
func TestService_FloodLedger(t *testing.T) {
	s := newSer(t, 2, testInterval)
	defer s.local.CloseAll()

	// Store the latest block
	reply, err := skipchain.NewClient().GetUpdateChain(s.sb.Roster, s.sb.SkipChainID())
	require.Nil(t, err)
	before := reply.Update[len(reply.Update)-1]

	log.Lvl1("Create 10 transactions and don't wait")
	for i := 0; i < 10; i++ {
		sendTransaction(t, s, 0, slowContract, 0)
	}
	// Send a last transaction and wait for it to be included
	sendTransaction(t, s, 0, dummyContract, 100)

	// Suppose we need at least 2 blocks (slowContract waits 1/5 interval for each execution)
	reply, err = skipchain.NewClient().GetUpdateChain(s.sb.Roster, s.sb.SkipChainID())
	require.Nil(t, err)
	latest := reply.Update[len(reply.Update)-1]
	if latest.Index-before.Index < 2 {
		t.Fatalf("didn't get at least 2 blocks: index before %d, index after %v", before.Index, latest.Index)
	}
}

func TestService_BigTx(t *testing.T) {
	// Use 1 second block interval for this test, as sending around these big blocks
	// gets to be too close to the edge with the normal 100ms testing interval, and
	// starts generating errors-that-might-not-be-errors.
	s := newSer(t, 1, 1*time.Second)
	defer s.local.CloseAll()

	// Check block number before.
	reply, err := skipchain.NewClient().GetUpdateChain(s.sb.Roster, s.sb.SkipChainID())
	require.Nil(t, err)
	latest := reply.Update[len(reply.Update)-1]
	require.Equal(t, 0, latest.Index)

	log.Lvl1("Create 2 giant transactions and 1 little one, wait for the 3rd one")
	_, e1, e2 := sendTransaction(t, s, 0, giantContract, 0)
	require.NoError(t, e1)
	require.NoError(t, e2)
	_, e1, e2 = sendTransaction(t, s, 0, giantContract, 0)
	require.NoError(t, e1)
	require.NoError(t, e2)
	p, e1, e2 := sendTransaction(t, s, 0, dummyContract, 2)
	require.NoError(t, e1)
	require.NoError(t, e2)
	require.True(t, p.InclusionProof.Match())

	// expect that the 2 last txns went into block #2.
	require.Equal(t, 2, p.Latest.Index)

	txr, err := txResultsFromBlock(&p.Latest)
	require.NoError(t, err)
	require.Equal(t, 2, len(txr))
}

func sendTransaction(t *testing.T, s *ser, client int, kind string, wait int) (Proof, error, error) {
	tx, err := createOneClientTx(s.darc.GetBaseID(), kind, s.value, s.signer)
	require.Nil(t, err)
	ser := s.services[client]
	_, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.sb.SkipChainID(),
		Transaction:   tx,
		InclusionWait: wait,
	})

	rep, err2 := ser.GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.sb.SkipChainID(),
		Key:     tx.Instructions[0].Hash(),
	})
	return rep.Proof, err, err2
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
	tx1, err := createOneClientTx(s.darc.GetBaseID(), invalidContract, value1, s.signer)
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
	tx2, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, value2, s.signer)
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.sb.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 10,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

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

	// TODO: This sleep is required for the same reason as the problem
	// documented in TestService_CloseAllDeadlock. How to fix it correctly?
	time.Sleep(2 * s.interval)
}

func findTx(tx ClientTransaction, res TxResults) TxResult {
	h := tx.Instructions.Hash()
	for i := range res {
		if bytes.Equal(res[i].ClientTransaction.Instructions.Hash(), h) {
			return res[i]
		}
	}
	panic("not found")
}

func TestService_LoadBlockInfo(t *testing.T) {
	interval := 200 * time.Millisecond
	s := newSer(t, 1, interval)
	defer s.local.CloseAll()

	dur, sz, err := s.service().LoadBlockInfo(s.sb.SkipChainID())
	require.Nil(t, err)
	require.Equal(t, dur, interval)
	require.True(t, sz == defaultMaxBlockSize)
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

		zeroBuf := make([]byte, 8)
		switch inst.GetType() {
		case SpawnType:
			// create the object if it doesn't exist
			if inst.Spawn.ContractID != "add" {
				return nil, nil, errors.New("can only spawn add contracts")
			}
			binary.PutVarint(zeroBuf, 0)
			return []StateChange{
				StateChange{
					StateAction: Create,
					InstanceID:  inst.DeriveID("add").Slice(),
					ContractID:  []byte(cid),
					Value:       zeroBuf,
				},
			}, nil, nil

		case InvokeType:
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
		return nil, nil, errors.New("need spawn or invoke")
	}
	RegisterContract(s.hosts[0], "add", f)

	cdb := s.service().getCollection(s.sb.SkipChainID())
	require.NotNil(t, cdb)

	// Manually create the add contract
	inst := genID()
	err := cdb.StoreAll([]StateChange{{
		StateAction: Create,
		InstanceID:  inst.Slice(),
		ContractID:  []byte("add"),
		Value:       make([]byte, 8),
	}}, 0)
	require.Nil(t, err)

	n := 5
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
			instrs[i].InstanceID = instrs[0].DeriveID("add")
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

	ct1 := ClientTransaction{Instructions: instrs}
	ct2 := ClientTransaction{Instructions: instrs2}

	_, txOut, scs := s.service().createStateChanges(cdb.coll, s.sb.SkipChainID(), NewTxResults(ct1, ct2), noTimeout)
	require.Equal(t, 2, len(txOut))
	require.True(t, txOut[0].Accepted)
	require.False(t, txOut[1].Accepted)
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
			InstanceID: NewInstanceID(s.darc.GetBaseID()),
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
	pr := s.waitProof(t, NewInstanceID(darc2.GetBaseID()))
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
			InstanceID: NewInstanceID(s.darc.GetBaseID()),
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
	pr := s.waitProof(t, NewInstanceID(darc2.GetBaseID()))
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
			InstanceID: NewInstanceID(darc2.GetBaseID()),
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
	pr = s.waitProof(t, NewInstanceID(darc3.GetBaseID()))
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

	ctx, newConfig := createConfigTx(t, s, false, false)
	s.sendTx(t, ctx)

	// wait for a change
	i := 0
	for ; i < 5; i++ {
		time.Sleep(s.interval)
		config, err := s.service().LoadConfig(s.sb.SkipChainID())
		require.NoError(t, err)

		if config.BlockInterval == newConfig.BlockInterval {
			break
		}
	}
	if i == 5 {
		require.Fail(t, "did not find new config in time")
	}

	interval, maxsz, err := s.service().LoadBlockInfo(s.sb.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, interval, 420*time.Millisecond)
	require.Equal(t, maxsz, 424242)
}

func TestService_SetBadConfig(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// send in a bad new block size
	ctx, badConfig := createConfigTx(t, s, false, true)
	s.sendTx(t, ctx)

	// wait for a change, which should not happen
	for i := 0; i < 5; i++ {
		time.Sleep(s.interval)
		config, err := s.service().LoadConfig(s.sb.SkipChainID())
		require.NoError(t, err)

		if badConfig.Roster.List[0].Equal(config.Roster.List[0]) {
			require.Fail(t, "found a bad config")
		}
	}

	// send in a bad new interval
	ctx, badConfig = createConfigTx(t, s, true, false)
	s.sendTx(t, ctx)

	// wait for a change, which should not happen
	for i := 0; i < 5; i++ {
		time.Sleep(s.interval)
		config, err := s.service().LoadConfig(s.sb.SkipChainID())
		require.NoError(t, err)

		if badConfig.Roster.List[0].Equal(config.Roster.List[0]) {
			require.Fail(t, "found a bad config")
		}
	}
}

// TestService_ViewChange is an end-to-end test for view-change. We kill the
// first nFailures nodes, where the nodes at index 0 is the current leader. The
// node at index nFailures should become the new leader. Then, we try to send a
// transaction to a follower, at index nFailures+1. The new leader (at index
// nFailures) should poll for new transactions and eventually make a new block
// containing that transaction. The new transaction should be stored on all
// followers. Finally, we bring the failed nodes back up and they should
// contain the transactions that they missed.
func TestService_ViewChange(t *testing.T) {
	testViewChange(t, 4, 1, 2*time.Second)
}

func TestService_ViewChange2(t *testing.T) {
	if testing.Short() {
		t.Skip("doesn't work on travis correctly due to byzcoinx timeout issue, see #1428")
	}
	testViewChange(t, 7, 2, 2*time.Second)
}

func testViewChange(t *testing.T, nHosts, nFailures int, interval time.Duration) {
	s := newSerN(t, 1, interval, nHosts, true)
	defer s.local.CloseAll()

	for _, service := range s.services {
		service.SetPropagationTimeout(2 * interval)
	}

	// Wait for all the genesis config to be written on all nodes.
	genesisInstanceID := InstanceID{}
	for i := range s.services {
		s.waitProofWithIdx(t, genesisInstanceID.Slice(), i)
	}

	// Stop the first nFailures hosts then the node at index nFailures
	// should take over.
	for i := 0; i < nFailures; i++ {
		log.Lvl1("stopping node at index", i)
		s.services[i].TestClose()
		s.hosts[i].Pause()
	}

	// Wait for proof that the new expected leader, s.services[nFailures],
	// has taken over. First, we sleep for the duration that an honest node
	// will wait before starting a view-change. Then, we sleep a little
	// longer for the view-change transaction to be stored in the block.
	for i := 1; i <= nFailures; i++ {
		time.Sleep(time.Duration(math.Pow(2, float64(i))) * s.interval * rotationWindow)
	}
	time.Sleep(interval)
	config, err := s.services[nFailures].LoadConfig(s.sb.SkipChainID())
	require.NoError(t, err)
	require.True(t, config.Roster.List[0].Equal(s.services[nFailures].ServerIdentity()))

	// check that the leader is updated for all nodes
	for _, service := range s.services[nFailures:] {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(s.sb.SkipChainID())
		require.NoError(t, err)
		require.NotNil(t, leader)
		require.True(t, leader.Equal(s.services[nFailures].ServerIdentity()))
	}

	// try to send a transaction to the node on index nFailures+1, which is
	// a follower (not the new leader)
	tx1, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.NoError(t, err)
	s.sendTxTo(t, tx1, nFailures+1)

	// wait for the transaction to be stored on the new leader, because it
	// polls for new transactions
	pr := s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID.Slice(), nFailures)
	require.True(t, pr.InclusionProof.Match())

	// The transaction should also be stored on followers
	for i := nFailures + 1; i < nHosts; i++ {
		pr = s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID.Slice(), i)
		require.True(t, pr.InclusionProof.Match())
	}

	// We need to bring the failed (the first nFailures) nodes back up and
	// check that they can synchronise to the latest state.
	for i := 0; i < nFailures; i++ {
		log.Lvl1("starting node at index", i)
		s.services[i].closed = false
		s.hosts[i].Unpause()
		require.NoError(t, s.services[i].startAllChains())
	}
	for i := 0; i < nFailures; i++ {
		pr = s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID.Slice(), i)
		require.True(t, pr.InclusionProof.Match())
	}
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
		require.NoError(t, service.startAllChains())
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
	collDB := s.service().getCollection(scID)
	collDB.StoreAll([]StateChange{{
		StateAction: Create,
		InstanceID:  NewInstanceID(s.darc.GetBaseID()).Slice(),
		ContractID:  []byte(contractID),
		Value:       []byte{},
	}}, 0)
	coll := collDB.coll
	tx1, err := createOneClientTx(s.darc.GetBaseID(), contractID, []byte{}, s.signer)
	require.Nil(t, err)

	// Add a second tx that is invalid because it is for an unknown contract.
	tx2, err := createOneClientTx(s.darc.GetBaseID(), contractID+"x", []byte{}, s.signer)
	require.Nil(t, err)

	txs := NewTxResults(tx1, tx2)
	require.NoError(t, err)
	root, txOut, states := s.service().createStateChanges(coll, scID, txs, noTimeout)
	require.Equal(t, 2, len(txOut))
	require.Equal(t, 0, len(states))
	require.Equal(t, 1, ctr)

	// If we call createStateChanges on the new txOut (as it will happen in production
	// when the tx set is reduced by the selection step, and then txOut are sent to
	// createStateChanges when making the block), then it should load it from the
	// cache, which means that ctr is still one (we do not call the
	// contract twice).
	root1, txOut1, states1 := s.service().createStateChanges(coll, scID, txOut, noTimeout)
	require.Equal(t, 1, ctr)
	require.Equal(t, root, root1)
	require.Equal(t, txOut, txOut1)
	require.Equal(t, states, states1)

	// If we remove the cache, then we expect the contract to be called
	// again, i.e., ctr == 2.
	s.service().stateChangeCache = newStateChangeCache()
	require.NoError(t, err)
	root2, txOut2, states2 := s.service().createStateChanges(coll, scID, txs, noTimeout)
	require.Equal(t, root, root2)
	require.Equal(t, txOut, txOut2)
	require.Equal(t, states, states2)
	require.Equal(t, 2, ctr)
}

func createConfigTx(t *testing.T, s *ser, intervalBad, szBad bool) (ClientTransaction, ChainConfig) {
	var config ChainConfig
	switch {
	case intervalBad:
		config = ChainConfig{-1, *s.roster.RandomSubset(s.services[1].ServerIdentity(), 2), defaultMaxBlockSize}
	case szBad:
		config = ChainConfig{420 * time.Millisecond, *s.roster.RandomSubset(s.services[1].ServerIdentity(), 2), 30 * 1e6}
	default:
		config = ChainConfig{420 * time.Millisecond, *s.roster, 424242}
	}
	configBuf, err := protobuf.Encode(&config)
	require.NoError(t, err)

	ctx := ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: NewInstanceID(nil),
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
		InstanceID: NewInstanceID(d2.GetBaseID()),
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
	return s.waitProofWithIdx(t, id.Slice(), 0)
}

func (s *ser) waitProofWithIdx(t *testing.T, key []byte, idx int) Proof {
	var pr Proof
	var ok bool
	for i := 0; i < 10; i++ {
		// wait for the block to be processed
		time.Sleep(2 * s.interval)

		resp, err := s.services[idx].GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     key,
			ID:      s.sb.SkipChainID(),
		})
		require.Nil(t, err)
		pr = resp.Proof
		if pr.InclusionProof.Match() {
			ok = true
			break
		}
	}

	require.True(t, ok, "got not match")
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
	return newSerN(t, step, interval, 4, false)
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

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster,
		[]string{"spawn:dummy", "spawn:invalid", "spawn:panic", "spawn:darc", "spawn:giant", "invoke:update_config", "spawn:slow", "spawn:stateShangeCacheTest", "delete"}, s.signer.Identity())
	require.Nil(t, err)
	s.darc = &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = interval
	s.interval = genesisMsg.BlockInterval

	for i := 0; i < step; i++ {
		switch i {
		case 0:
			resp, err := s.service().CreateGenesisBlock(genesisMsg)
			// The problem here is that the config is being created with MaxBloclSize == 0. Why? Where?
			require.Nil(t, err)
			s.sb = resp.Skipblock
		case 1:
			tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
			require.Nil(t, err)
			s.tx = tx
			_, err = s.service().AddTransaction(&AddTxRequest{
				Version:       CurrentVersion,
				SkipchainID:   s.sb.SkipChainID(),
				Transaction:   tx,
				InclusionWait: 10,
			})
			require.Nil(t, err)
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
	err := inst.VerifyDarcSignature(cdb)
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
			NewStateChange(Create, NewInstanceID(inst.Hash()), inst.Spawn.ContractID, inst.Spawn.Args[0].Value, darcID),
		}, nil, nil
	case DeleteType:
		return []StateChange{
			NewStateChange(Remove, inst.InstanceID, "", nil, darcID),
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

func giantContractFunc(cdb CollectionView, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	err := inst.VerifyDarcSignature(cdb)
	if err != nil {
		return nil, nil, err
	}

	_, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	// stuff itself fills up 1/4 of a block.
	stuff := make([]byte, defaultMaxBlockSize/4)

	switch inst.GetType() {
	case SpawnType:
		return []StateChange{
			// Send stuff 3 times, so that this txn fills up 3/4 of a block.
			NewStateChange(Create, NewInstanceID(inst.Hash()), inst.Spawn.ContractID, stuff, darcID),
			NewStateChange(Create, inst.DeriveID("one"), inst.Spawn.ContractID, stuff, darcID),
			NewStateChange(Create, inst.DeriveID("two"), inst.Spawn.ContractID, stuff, darcID),
		}, nil, nil
	default:
		panic("should not get here")
	}
}

func registerDummy(servers []*onet.Server) {
	// For testing - there must be a better way to do that. But putting
	// services []skipchain.GetService in the method signature doesn't work :(
	for _, s := range servers {
		RegisterContract(s, dummyContract, dummyContractFunc)
		RegisterContract(s, slowContract, slowContractFunc)
		RegisterContract(s, giantContract, giantContractFunc)
		RegisterContract(s, invalidContract, invalidContractFunc)
	}
}

func genID() (i InstanceID) {
	random.Bytes(i[:], random.New())
	return i
}

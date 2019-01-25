package byzcoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/darc/expression"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/sign/eddsa"
	"go.dedis.ch/kyber/suites"
	"go.dedis.ch/kyber/util/random"
)

var tSuite = suites.MustFind("Ed25519")
var dummyContract = "dummy"
var slowContract = "slow"
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
	k, _, _, _, err := proof.Proof.KeyValue()
	require.Nil(t, err)
	require.EqualValues(t, genesisMsg.GenesisDarc.GetID(), k)

	interval, maxsz, err := s.service().LoadBlockInfo(resp.Skipblock.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, interval, genesisMsg.BlockInterval)
	require.Equal(t, maxsz, genesisMsg.MaxBlockSize)
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
	log.SetShowTime(true)
	var s *ser
	if failure {
		s = newSerN(t, 1, time.Second, 4, false)
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
		SkipchainID: s.genesis.SkipChainID(),
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
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, 1)
	require.Nil(t, err)
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 5,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// add the second tx
	log.Lvl1("adding the second tx")
	value2 := []byte("value2")
	tx2, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, value2, s.signer, 2)
	require.Nil(t, err)
	akvresp, err = s.services[sendToIdx].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 5,
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
			s.service().TestClose()
			require.NoError(t, s.service().startAllChains())
		}
		for _, tx := range txs {
			pr := s.waitProofWithIdx(t, tx.Instructions[0].Hash(), 0)
			require.Nil(t, pr.Verify(s.genesis.SkipChainID()))
			_, v0, _, _, err := pr.KeyValue()
			require.Nil(t, err)
			require.True(t, bytes.Equal(tx.Instructions[0].Spawn.Args[0].Value, v0))

			// check that the database has this new block's index recorded
			st, err := s.services[0].getStateTrie(pr.Latest.SkipChainID())
			require.NoError(t, err)
			idx := st.GetIndex()
			require.Equal(t, pr.Latest.Index, idx)
		}
	}

	// Bring the failed node back up and it should also see the transactions.
	if failure {
		log.Lvl1("bringing the failed node back up")
		s.hosts[len(s.hosts)-1].Unpause()
		require.NoError(t, s.services[len(s.hosts)-1].startAllChains())

		for _, tx := range txs {
			pr := s.waitProofWithIdx(t, tx.Instructions[0].Hash(), len(s.hosts)-1)
			require.Nil(t, pr.Verify(s.genesis.SkipChainID()))
			_, v0, _, _, err := pr.KeyValue()
			require.Nil(t, err)
			require.True(t, bytes.Equal(tx.Instructions[0].Spawn.Args[0].Value, v0))
			// check that the database has this new block's index recorded
			st, err := s.services[len(s.hosts)-1].getStateTrie(pr.Latest.SkipChainID())
			require.NoError(t, err)
			idx := st.GetIndex()
			require.Equal(t, pr.Latest.Index, idx)
		}

		// Try to add a new transaction to the node that failed (but is
		// now running) and it should work.
		log.Lvl1("making a last transaction")
		pr, k, err, err2 := sendTransaction(t, s, len(s.hosts)-1, dummyContract, 10)
		require.NoError(t, err)
		require.NoError(t, err2)
		require.True(t, pr.InclusionProof.Match(k))

		log.Lvl1("done")
		// Wait for tasks to finish.
		time.Sleep(time.Second)
	}
}

func TestService_AddTransaction_WrongNode(t *testing.T) {
	log.SetShowTime(true)
	s := newSerN(t, 1, time.Second, 4, false)
	defer s.local.CloseAll()

	outsideServer := s.local.GenServers(1)[0]
	outside := outsideServer.Service(ServiceName).(*Service)
	registerDummy([]*onet.Server{outsideServer})

	// add the first tx to outside server
	log.Lvl1("adding the first tx - this should fail")
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, 1)
	require.Nil(t, err)
	atx := &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 5,
	}
	_, err = outside.AddTransaction(atx)
	require.NotNil(t, err)

	// Adding outside to roster
	log.Lvl1("Adding new node to the roster")
	rosterR := onet.NewRoster(append(s.roster.List, outside.ServerIdentity()))
	ctx, _ := createConfigTxWithCounter(t, testInterval, *rosterR, defaultMaxBlockSize, s, 1)
	s.sendTxAndWait(t, ctx, 10)

	// force the synchronization as the new node needs to get the propagation
	// to know about the skipchain but we're not testing that here
	proof, err := s.service().db().GetProof(s.genesis.Hash)
	require.NoError(t, err)
	outside.db().StoreBlocks(proof)

	log.Lvl1("adding tx to now included node")
	atx.Transaction, err = createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, 2)
	require.Nil(t, err)
	_, err = outside.AddTransaction(atx)
	require.Nil(t, err)
}

// Tests what happens if a transaction with two instructions is sent: one valid and one invalid
// instruction.
func TestService_AddTransaction_ValidInvalid(t *testing.T) {
	log.SetShowTime(true)
	s := newSerN(t, 1, time.Second, 4, false)
	defer s.local.CloseAll()

	// add the first tx to create the instance
	log.Lvl1("Adding the first tx")
	dcID := random.Bits(256, false, random.New())
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, dcID, s.signer, 1)
	require.Nil(t, err)
	atx := &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 5,
	}
	_, err = s.service().AddTransaction(atx)
	require.Nil(t, err)

	// add a second tx that holds two instructions: one valid and one invalid (creates the same contract)
	log.Lvl1("Adding the second tx")
	instr1 := createInvokeInstr(NewInstanceID(dcID), "evolve", "data", dcID)
	instr1.SignerCounter = []uint64{2}
	instr2 := createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", dcID)
	instr2.SignerCounter = []uint64{3}
	tx2 := ClientTransaction{
		Instructions: []Instruction{instr1, instr2},
	}
	h := tx2.Instructions.Hash()
	for i := range tx2.Instructions {
		err := tx2.Instructions[i].SignWith(h, s.signer)
		require.Nil(t, err)
	}
	atx = &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 5,
	}
	_, err = s.service().AddTransaction(atx)
	require.NotNil(t, err)

	// add a third tx that holds two valid instructions
	log.Lvl1("Adding a third, valid tx")
	instr1 = createInvokeInstr(NewInstanceID(dcID), "evolve", "data", dcID)
	instr1.SignerCounter = []uint64{2}
	dcID2 := random.Bits(256, true, random.New())
	instr2 = createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", dcID2)
	instr2.SignerCounter = []uint64{3}
	tx3 := ClientTransaction{
		Instructions: []Instruction{instr1, instr2},
	}
	h = tx3.Instructions.Hash()
	for i := range tx3.Instructions {
		err := tx3.Instructions[i].SignWith(h, s.signer)
		require.Nil(t, err)
	}
	atx = &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx3,
		InclusionWait: 5,
	}
	_, err = s.service().AddTransaction(atx)
	require.Nil(t, err)
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
			ID:      s.genesis.SkipChainID(),
			Key:     serKey,
		})
		require.Nil(t, err)
		if rep.Proof.InclusionProof.Match(serKey) {
			break
		}
	}
	require.NotEqual(t, 10, i, "didn't get proof in time")
	key, v0, _, _, err := rep.Proof.KeyValue()
	require.Equal(t, key, serKey)
	require.Nil(t, err)
	require.Nil(t, rep.Proof.Verify(s.genesis.SkipChainID()))
	require.Equal(t, serKey, key)
	require.Equal(t, s.value, v0)

	// Modify the key and we should not be able to get the proof.
	wrongKey := append(serKey, byte(0))
	rep, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.SkipChainID(),
		Key:     wrongKey,
	})
	require.NoError(t, err)
	require.NoError(t, rep.Proof.Verify(s.genesis.SkipChainID()))
	_, _, _, err = rep.Proof.Get(wrongKey)
	require.Error(t, err)
}

func TestService_DarcProxy(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	email := "test@example.com"
	ed := eddsa.NewEdDSA(cothority.Suite.RandomStream())

	// signer with placeholder callback while we find out the Id string
	signer := darc.NewSignerProxy(email, ed.Public, nil)
	id := signer.Identity()

	// Evolve the genesis Darc to have a rule for OpenID signing
	d2 := s.darc.Copy()
	require.Nil(t, d2.EvolveFrom(s.darc))
	err := d2.Rules.UpdateRule("spawn:dummy", expression.Expr(id.String()))
	require.NoError(t, err)
	s.testDarcEvolution(t, *d2, false)

	ga := func(msg []byte) ([]byte, error) {
		h := sha256.New()
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(len(email)))
		h.Write(b)
		h.Write([]byte(email))
		h.Write(msg)
		msg2 := h.Sum(nil)

		// In this simulation, we can make a signature the simple way: eddsa.Sign
		// With auth proxies which are using DSS, the client will contact proxies
		// to get signatures, then interpolate them into the final signature.
		return ed.Sign(msg2)
	}

	// now set the signer with the correct callback
	signer = darc.NewSignerProxy(email, ed.Public, ga)
	ctx := ClientTransaction{
		Instructions: []Instruction{
			{
				InstanceID: NewInstanceID(d2.GetBaseID()),
				Spawn: &Spawn{
					ContractID: "dummy",
					Args:       Arguments{{Name: "data", Value: []byte("nothing in particular")}},
				},
				SignerCounter: []uint64{1},
			},
		},
	}

	err = ctx.SignWith(signer)
	require.Nil(t, err)

	_, err = s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: 10,
	})
	require.Nil(t, err)
}

func TestService_WrongSigner(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	in1 := createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", []byte("whatever"))
	in1.SignerCounter = []uint64{1}

	signer := darc.NewSignerEd25519(nil, nil)
	tx, err := combineInstrsAndSign(signer, in1)
	require.NoError(t, err)

	_, err = s.services[0].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 2,
	})
	// Expect it to not be accepted, because only s.signer is in the Darc
	require.Error(t, err)
}

// Test that inter-instruction dependencies are correctly handled.
func TestService_Depending(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Create a client tx with two instructions in it where the second one depends on
	// the first one having executed.

	// First instruction: spawn a dummy value.
	in1 := createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", []byte("something to delete"))
	in1.SignerCounter = []uint64{1}

	// Second instruction: delete the value we just spawned.
	in2 := Instruction{
		InstanceID: NewInstanceID(in1.Hash()),
		Delete:     &Delete{},
	}
	in2.SignerCounter = []uint64{2}

	tx, err := combineInstrsAndSign(s.signer, in1, in2)
	require.NoError(t, err)

	_, err = s.services[0].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 2,
	})
	require.Nil(t, err)

	cdb, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	_, _, _, _, err = cdb.GetValues(in1.Hash())
	require.Error(t, err)
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
	oldmtw := minTimestampWindow
	defer func() {
		minTimestampWindow = oldmtw
	}()
	minTimestampWindow = time.Second

	// Hook the verifier in order delay the arrival and test timestamp checking.
	ser := s.services[0]
	c := ser.Context
	err := skipchain.RegisterVerification(c, verifyByzCoin, func(newID []byte, newSB *skipchain.SkipBlock) bool {
		// Make this block arrive late compared to it's timestamp. The window will be
		// 1000ms, so sleep 1200 more, just to be sure.
		time.Sleep(2200 * time.Millisecond)
		return ser.verifySkipBlock(newID, newSB)
	})
	require.Nil(t, err)

	tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.Nil(t, err)
	_, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 5,
	})
	require.Error(t, err)
	log.Lvl1("Last test OK")
}

func TestService_BadDataHeader(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	ser := s.services[0]
	c := ser.Context
	err := skipchain.RegisterVerification(c, verifyByzCoin, func(newID []byte, newSB *skipchain.SkipBlock) bool {
		// Hack up the DataHeader to make the TrieRoot the wrong size.
		var header DataHeader
		err := protobuf.DecodeWithConstructors(newSB.Data, &header, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			t.Fatal(err)
		}
		header.TrieRoot = append(header.TrieRoot, 0xff)
		newSB.Data, _ = protobuf.Encode(header)

		return ser.verifySkipBlock(newID, newSB)
	})
	require.Nil(t, err)

	tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.Nil(t, err)
	_, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
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
	n := 3
	if testing.Short() {
		n = 1
	}

	for i := 0; i < n; i++ {
		log.Lvl1("Testing inclusion when sending to service", i)
		waitInclusion(t, i)
	}
}

func waitInclusion(t *testing.T, client int) {
	s := newSer(t, 2, testInterval)
	defer s.local.CloseAll()

	// Create a transaction without waiting
	log.Lvl1("Create transaction and don't wait")
	pr, k, err, err2 := sendTransaction(t, s, client, dummyContract, 0)
	require.NoError(t, err)
	require.NoError(t, err2)
	require.False(t, pr.InclusionProof.Match(k))

	log.Lvl1("Create correct transaction and wait")
	pr, k, err, err2 = sendTransaction(t, s, client, dummyContract, 10)
	require.NoError(t, err)
	require.NoError(t, err2)
	require.True(t, pr.InclusionProof.Match(k))

	// We expect to see both transactions in the block in pr.
	txr, err := txResultsFromBlock(&pr.Latest)
	require.NoError(t, err)
	require.Equal(t, len(txr), 2)

	log.Lvl1("Create wrong transaction and wait")
	pr, _, err, err2 = sendTransaction(t, s, client, invalidContract, 10)
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
	pr, k, err, err2 = sendTransaction(t, s, client, dummyContract, 10)
	require.NoError(t, err)
	require.NoError(t, err2)
	require.True(t, pr.InclusionProof.Match(k))

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

	// Fetch the latest block
	reply, err := skipchain.NewClient().GetUpdateChain(s.genesis.Roster, s.genesis.SkipChainID())
	require.Nil(t, err)
	before := reply.Update[len(reply.Update)-1]

	log.Lvl1("Create 10 transactions and don't wait")
	n := 10
	for i := 0; i < n; i++ {
		sendTransactionWithCounter(t, s, 0, slowContract, 0, uint64(i)+1)
	}
	// Send a last transaction and wait for it to be included
	sendTransactionWithCounter(t, s, 0, dummyContract, 100, uint64(n)+2)

	// Suppose we need at least 2 blocks (slowContract waits 1/5 interval for each execution)
	reply, err = skipchain.NewClient().GetUpdateChain(s.genesis.Roster, s.genesis.SkipChainID())
	require.Nil(t, err)
	latest := reply.Update[len(reply.Update)-1]
	if latest.Index-before.Index < 2 {
		t.Fatalf("didn't get at least 2 blocks: index before %d, index after %v", before.Index, latest.Index)
	}
}

func TestService_BigTx(t *testing.T) {
	// Use longer block interval for this test, as sending around these big blocks
	// gets to be too close to the edge with the normal short testing interval, and
	// starts generating errors-that-might-not-be-errors.
	s := newSer(t, 1, 1*time.Second)
	defer s.local.CloseAll()

	// Check block number before.
	reply, err := skipchain.NewClient().GetUpdateChain(s.genesis.Roster, s.genesis.SkipChainID())
	require.Nil(t, err)
	latest := reply.Update[len(reply.Update)-1]
	require.Equal(t, 0, latest.Index)

	save := s.value

	// Try to send a value so big it will be refused.
	s.value = make([]byte, defaultMaxBlockSize+1)
	_, _, e1, e2 := sendTransaction(t, s, 0, dummyContract, 0)
	require.Error(t, e1)
	require.Contains(t, "transaction too large", e1.Error())
	require.NoError(t, e2)

	// Now send values that are 3/4 as big as one block.
	s.value = make([]byte, defaultMaxBlockSize/4*3)

	log.Lvl1("Create 2 giant transactions and 1 little one, wait for the 3rd one")
	_, _, e1, e2 = sendTransactionWithCounter(t, s, 0, dummyContract, 0, 1)
	require.NoError(t, e1)
	require.NoError(t, e2)
	_, _, e1, e2 = sendTransactionWithCounter(t, s, 0, dummyContract, 0, 2)
	require.NoError(t, e1)
	require.NoError(t, e2)

	// Back to little values again for the last tx.
	s.value = save
	p, k, e1, e2 := sendTransactionWithCounter(t, s, 0, dummyContract, 10, 3)
	require.NoError(t, e1)
	require.NoError(t, e2)
	require.True(t, p.InclusionProof.Match(k))

	// expect that the 2 last txns went into block #2.
	require.Equal(t, 2, p.Latest.Index)

	txr, err := txResultsFromBlock(&p.Latest)
	require.NoError(t, err)
	require.Equal(t, 2, len(txr))
}

func sendTransaction(t *testing.T, s *ser, client int, kind string, wait int) (Proof, []byte, error, error) {
	counterResponse, err := s.service().GetSignerCounters(&GetSignerCounters{
		SignerIDs:   []string{s.signer.Identity().String()},
		SkipchainID: s.genesis.SkipChainID(),
	})
	require.NoError(t, err)
	return sendTransactionWithCounter(t, s, client, kind, wait, counterResponse.Counters[0]+1)
}

func sendTransactionWithCounter(t *testing.T, s *ser, client int, kind string, wait int, counter uint64) (Proof, []byte, error, error) {
	tx, err := createOneClientTxWithCounter(s.darc.GetBaseID(), kind, s.value, s.signer, counter)
	require.Nil(t, err)
	ser := s.services[client]
	_, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: wait,
	})

	rep, err2 := ser.GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.SkipChainID(),
		Key:     tx.Instructions[0].Hash(),
	})
	return rep.Proof, tx.Instructions[0].Hash(), err, err2
}

func TestService_InvalidVerification(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	for i := range s.hosts {
		RegisterContract(s.hosts[i], "panic", adaptor(panicContractFunc))
	}

	// tx0 uses the panicing contract, so it should _not_ be stored.
	value1 := []byte("a")
	tx0, err := createOneClientTx(s.darc.GetBaseID(), "panic", value1, s.signer)
	require.Nil(t, err)
	akvresp, err := s.service().AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.genesis.SkipChainID(),
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
		SkipchainID: s.genesis.SkipChainID(),
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
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 10,
	})
	require.Nil(t, err)
	require.NotNil(t, akvresp)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// Check that tx1 is _not_ stored.
	pr, err := s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.SkipChainID(),
		Key:     tx1.Instructions[0].Hash(),
	})
	require.Nil(t, err)
	match := pr.Proof.InclusionProof.Match(tx1.Instructions[0].Hash())
	require.False(t, match)

	// Check that tx2 is stored.
	pr, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.SkipChainID(),
		Key:     tx2.Instructions[0].Hash(),
	})
	require.Nil(t, err)
	match = pr.Proof.InclusionProof.Match(tx2.Instructions[0].Hash())
	require.True(t, match)

	// TODO: This sleep is required for the same reason as the problem
	// documented in TestService_CloseAllDeadlock. How to fix it correctly?
	time.Sleep(2 * s.interval)
}

func TestService_LoadBlockInfo(t *testing.T) {
	interval := 200 * time.Millisecond
	s := newSer(t, 1, interval)
	defer s.local.CloseAll()

	dur, sz, err := s.service().LoadBlockInfo(s.genesis.SkipChainID())
	require.Nil(t, err)
	require.Equal(t, dur, interval)
	require.True(t, sz == defaultMaxBlockSize)
}

func TestService_StateChange(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	var latest int64
	f := func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
		_, _, cid, _, err := cdb.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return nil, nil, err
		}

		val0, _, _, _, err := cdb.GetValues(inst.InstanceID.Slice())
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
			v, _ := binary.Varint(val0)
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
	RegisterContract(s.hosts[0], "add", adaptorNoVerify(f))

	cdb, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, cdb)

	// Manually create the add contract
	inst := genID()
	err = cdb.StoreAll([]StateChange{{
		StateAction: Create,
		InstanceID:  inst.Slice(),
		ContractID:  []byte("add"),
		Value:       make([]byte, 8),
	}}, 0)
	require.Nil(t, err)

	n := 5
	instrs := make([]Instruction, n)
	for i := range instrs {
		instrs[i] = Instruction{
			InstanceID: inst,
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
		Spawn: &Spawn{
			ContractID: "not-add",
		},
	}

	ct1 := ClientTransaction{Instructions: instrs}
	ct2 := ClientTransaction{Instructions: instrs2}

	_, txOut, scs, _ := s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ct1, ct2), noTimeout)
	require.Equal(t, 2, len(txOut))
	require.True(t, txOut[0].Accepted)
	require.False(t, txOut[1].Accepted)
	require.Equal(t, n, len(scs))
	require.Equal(t, latest, int64(n-1))
}

func TestService_StateChangeVerification(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	cid := "createSC"
	iid := NewInstanceID(make([]byte, 32))
	iid[0] = byte(32)
	iid2 := NewInstanceID(iid.Slice())
	iid2[0] = byte(64)
	f := func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
		zeroBuf := make([]byte, 8)
		var sa StateAction
		switch inst.GetType() {
		case SpawnType:
			sa = Create
		case InvokeType:
			sa = Update
		case DeleteType:
			sa = Remove
		}

		return []StateChange{
			StateChange{
				StateAction: Create,
				InstanceID:  inst.DeriveID("").Slice(),
				ContractID:  []byte(cid),
				Value:       zeroBuf,
			},
			StateChange{
				StateAction: Update,
				InstanceID:  inst.DeriveID("").Slice(),
				ContractID:  []byte(cid),
				Value:       zeroBuf,
			},
			StateChange{
				StateAction: sa,
				InstanceID:  iid2.Slice(),
				ContractID:  []byte(cid),
				Value:       zeroBuf,
			},
		}, nil, nil
	}
	RegisterContract(s.hosts[0], cid, adaptorNoVerify(f))
	cdb, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, cdb)

	// Create iid so we can send instructions to it
	err = cdb.StoreAll([]StateChange{{
		StateAction: Create,
		InstanceID:  iid.Slice(),
		ContractID:  []byte(cid),
		Value:       make([]byte, 8),
	}}, 0)
	require.Nil(t, err)

	log.Lvl1("Failing updating and removing non-existing instances")
	_, txOut, scs, _ := s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Invoke:     &Invoke{},
	}}}), noTimeout)
	require.Equal(t, 0, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, false, txOut[0].Accepted)
	_, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Delete:     &Delete{},
	}}}), noTimeout)
	require.Equal(t, 0, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, false, txOut[0].Accepted)

	log.Lvl1("Create new instance, but fail to create it twice")
	_, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Spawn:      &Spawn{ContractID: cid},
	}}}), noTimeout)
	require.Equal(t, 3, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, true, txOut[0].Accepted)
	require.Nil(t, cdb.StoreAll(scs, 0))
	_, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Spawn:      &Spawn{ContractID: cid},
	}}}), noTimeout)
	require.Equal(t, 0, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, false, txOut[0].Accepted)

	log.Lvl1("Accept updating and removing existing instance")
	_, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Invoke:     &Invoke{},
	}}}), noTimeout)
	require.Equal(t, 3, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, true, txOut[0].Accepted)
	_, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Delete:     &Delete{},
	}}}), noTimeout)
	require.Equal(t, 3, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, true, txOut[0].Accepted)
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
	require.True(t, pr.InclusionProof.Match(d2.GetBaseID()))
	_, v0, _, _, err := pr.KeyValue()
	require.Nil(t, err)
	d22, err := darc.NewFromProtobuf(v0)
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
	require.True(t, pr.InclusionProof.Match(d2.GetBaseID()))
	_, v0, _, _, err := pr.KeyValue()
	require.Nil(t, err)
	d22, err := darc.NewFromProtobuf(v0)
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
			Spawn: &Spawn{
				ContractID: ContractDarcID,
				Args: []Argument{{
					Name:  "darc",
					Value: darc2Buf,
				}},
			},
			SignerCounter: []uint64{1},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignWith(ctx.Instructions.Hash(), s.signer))

	s.sendTx(t, ctx)
	pr := s.waitProof(t, NewInstanceID(darc2.GetBaseID()))
	require.True(t, pr.InclusionProof.Match(darc2.GetBaseID()))
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
	instr := Instruction{
		InstanceID: NewInstanceID(s.darc.GetBaseID()),
		Spawn: &Spawn{
			ContractID: ContractDarcID,
			Args: []Argument{{
				Name:  "darc",
				Value: darc2Buf,
			}},
		},
		SignerCounter: []uint64{1},
	}
	ctx, err := combineInstrsAndSign(s.signer, instr)
	s.sendTx(t, ctx)
	pr := s.waitProof(t, NewInstanceID(darc2.GetBaseID()))
	require.True(t, pr.InclusionProof.Match(darc2.GetBaseID()))

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
	instr = Instruction{
		InstanceID: NewInstanceID(darc2.GetBaseID()),
		Spawn: &Spawn{
			ContractID: ContractDarcID,
			Args: []Argument{{
				Name:  "darc",
				Value: darc3Buf,
			}},
		},
		SignerCounter: []uint64{2},
	}
	ctx, err = combineInstrsAndSign(s.signer, instr)
	require.NoError(t, err)
	s.sendTx(t, ctx)
	pr = s.waitProof(t, NewInstanceID(darc3.GetBaseID()))
	require.True(t, pr.InclusionProof.Match(darc3.GetBaseID()))
}

func TestService_CheckAuthorization(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Spawn second darc with a new owner/signer, but delegate its spawn
	// rule to the first darc
	signer2 := darc.NewSignerEd25519(nil, nil)
	id2 := []darc.Identity{signer2.Identity()}
	darc2 := darc.NewDarc(darc.InitRules(id2, id2),
		[]byte("second darc"))
	darc2.Rules.AddRule("spawn:darc", expression.Expr(s.darc.GetIdentityString()))
	darc2Buf, err := darc2.ToProto()
	require.Nil(t, err)
	darc2Copy, err := darc.NewFromProtobuf(darc2Buf)
	require.Nil(t, err)
	require.True(t, darc2.Equal(darc2Copy))
	instr := Instruction{
		InstanceID: NewInstanceID(s.darc.GetBaseID()),
		Spawn: &Spawn{
			ContractID: ContractDarcID,
			Args: []Argument{{
				Name:  "darc",
				Value: darc2Buf,
			}},
		},
		SignerCounter: []uint64{1},
	}
	ctx, err := combineInstrsAndSign(s.signer, instr)
	require.NoError(t, err)
	s.sendTx(t, ctx)
	pr := s.waitProof(t, NewInstanceID(darc2.GetBaseID()))
	require.True(t, pr.InclusionProof.Match(darc2.GetBaseID()))

	ca := &CheckAuthorization{
		Version:    CurrentVersion,
		ByzCoinID:  s.genesis.SkipChainID(),
		DarcID:     s.darc.GetBaseID(),
		Identities: []darc.Identity{s.signer.Identity()},
	}
	resp, err := s.service().CheckAuthorization(ca)
	require.Nil(t, err)
	require.Contains(t, resp.Actions, darc.Action("_sign"))

	ca.Identities[0] = darc.NewIdentityEd25519(s.roster.List[0].Public)
	resp, err = s.service().CheckAuthorization(ca)
	require.Nil(t, err)
	require.Contains(t, resp.Actions, darc.Action("invoke:view_change"))

	ca.Identities = append(ca.Identities, darc.NewIdentityEd25519(s.roster.List[1].Public))
	resp, err = s.service().CheckAuthorization(ca)
	require.Nil(t, err)
	require.Contains(t, resp.Actions, darc.Action("invoke:view_change"))

	log.Lvl1("Check delegation of darcs")
	ca.DarcID = darc2.GetID()
	ca.Identities[0] = darc.NewSignerEd25519(nil, nil).Identity()
	resp, err = s.service().CheckAuthorization(ca)
	require.Nil(t, err)
	require.Equal(t, 0, len(resp.Actions))

	ca.DarcID = darc2.GetID()
	ca.Identities[0] = s.signer.Identity()
	resp, err = s.service().CheckAuthorization(ca)
	require.Nil(t, err)
	require.Contains(t, resp.Actions, darc.Action("spawn:darc"))

	ca.DarcID = darc2.GetID()
	ca.Identities[0] = darc.NewIdentityDarc(s.darc.GetID())
	resp, err = s.service().CheckAuthorization(ca)
	require.Nil(t, err)
	require.Contains(t, resp.Actions, darc.Action("spawn:darc"))
}

func TestService_GetLeader(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	for _, service := range s.services {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(s.genesis.SkipChainID())
		require.NoError(t, err)
		require.NotNil(t, leader)
		require.True(t, leader.Equal(s.services[0].ServerIdentity()))
	}
}

func TestService_SetConfig(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	interval := 42 * time.Millisecond
	blocksize := 424242
	ctx, _ := createConfigTxWithCounter(t, interval, *s.roster, blocksize, s, 1)
	s.sendTxAndWait(t, ctx, 10)

	_, err := s.service().LoadConfig(s.genesis.SkipChainID())
	require.NoError(t, err)

	newInterval, newBlocksize, err := s.service().LoadBlockInfo(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, interval, newInterval)
	require.Equal(t, blocksize, newBlocksize)
}

func TestService_SetConfigInterval(t *testing.T) {
	defer log.SetShowTime(log.ShowTime())
	log.SetShowTime(true)
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Wait for a block completion to start the interval check
	// to prevent the first one to be included in the setup block
	ctx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer)
	require.Nil(t, err)
	s.sendTxAndWait(t, ctx, 10)

	intervals := []time.Duration{testInterval, testInterval / 5,
		testInterval * 2, testInterval / 5, testInterval * 20}
	if testing.Short() {
		intervals = intervals[0:3]
	}

	counter := 2
	for _, interval := range intervals {
		// The next block should now be in the range of testInterval.
		log.Lvl1("Setting interval to", interval)
		ctx, _ := createConfigTxWithCounter(t, interval, *s.roster, defaultMaxBlockSize, s, counter)
		counter++
		// The wait argument here is also used in case no block is received, so
		// it means: at most 10*blockInterval, or after 10 blocks, whichever comes
		// first. Putting it to 1 doesn't work, because the actual blockInterval
		// is bigger, due to dedis/cothority#1409
		s.sendTxAndWait(t, ctx, 10)

		start := time.Now()

		dummyCtx, _ := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(counter))
		counter++
		s.sendTxAndWait(t, dummyCtx, 10)

		dur := time.Now().Sub(start)
		require.InDelta(t, dur, interval, float64(1*time.Second))
	}
}

func TestService_SetConfigRosterKeepLeader(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	log.Lvl1("Creating blocks to check rotation of the roster while keeping leader")
	rosterR := s.roster
	for i := 0; i < 6; i++ {
		rosterR = onet.NewRoster([]*network.ServerIdentity{
			rosterR.List[0], rosterR.List[2], rosterR.List[3], rosterR.List[1]})
		log.Lvl2("Creating block", i)
		ctx, _ := createConfigTxWithCounter(t, testInterval, *rosterR, defaultMaxBlockSize, s, 1+i)
		s.sendTxAndWait(t, ctx, 10)
		log.Lvl2("Verifying the correct roster is in place")
		latest, err := s.service().db().GetLatestByID(s.genesis.Hash)
		require.Nil(t, err)
		require.True(t, latest.Roster.ID.Equal(rosterR.ID), "roster has not been updated")
	}
}

func TestService_SetConfigRosterNewLeader(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	log.Lvl1("Creating blocks to check rotation of the leader")
	rosterR := s.roster
	for i := 0; i < 1; i++ {
		rosterR = onet.NewRoster([]*network.ServerIdentity{
			rosterR.List[1], rosterR.List[2], rosterR.List[3], rosterR.List[0]})
		log.Lvl2("Creating block", i)
		ctx, _ := createConfigTxWithCounter(t, testInterval, *rosterR, defaultMaxBlockSize, s, 1+i)
		s.sendTxAndWait(t, ctx, 10)
		log.Lvl2("Verifying the correct roster is in place")
		latest, err := s.service().db().GetLatestByID(s.genesis.Hash)
		require.Nil(t, err)
		require.True(t, latest.Roster.ID.Equal(rosterR.ID), "roster has not been updated")
	}
}

func TestService_SetConfigRosterNewNodes(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	nbrNewNodes := 10
	if testing.Short() {
		nbrNewNodes = 5
	}

	servers, newRoster, _ := s.local.MakeSRS(cothority.Suite, nbrNewNodes, ByzCoinID)

	ids := []darc.Identity{s.signer.Identity()}
	testDarc := darc.NewDarc(darc.InitRules(ids, ids), []byte("testDarc"))
	testDarcBuf, err := testDarc.ToProto()
	require.Nil(t, err)
	instr := createSpawnInstr(s.darc.GetBaseID(), ContractDarcID, "darc", testDarcBuf)
	require.Nil(t, err)
	ctx, err := combineInstrsAndSign(s.signer, instr)
	require.Nil(t, err)
	s.sendTxAndWait(t, ctx, 10)

	log.Lvl1("Creating blocks to check rotation of the leader")
	leanClient := onet.NewClient(cothority.Suite, ServiceName)
	rosterR := s.roster
	counter := 2
	for _, newNode := range newRoster.List {
		var i int
		for i = 1; i < 10; i++ {
			time.Sleep(testInterval * time.Duration(i))
			log.Lvlf2("Verifying the last node %s has all the data", rosterR.List[len(rosterR.List)-1])
			reply := &GetProofResponse{}
			err = leanClient.SendProtobuf(rosterR.List[len(rosterR.List)-1], &GetProof{
				Version: CurrentVersion,
				ID:      s.genesis.Hash,
				Key:     testDarc.GetBaseID(),
			}, reply)
			if err == nil && reply.Proof.InclusionProof.Match(testDarc.GetBaseID()) {
				break
			}
		}
		require.True(t, i < 10, "didn't get proof in reasonable time")

		log.Lvlf2("Adding new node to the roster")
		rosterR = onet.NewRoster(append(rosterR.List, newNode))
		ctx, _ := createConfigTxWithCounter(t, testInterval, *rosterR, defaultMaxBlockSize, s, counter)
		counter++
		s.sendTxAndWait(t, ctx, 10)

		log.Lvl2("Verifying the correct roster is in place")
		latest, err := s.service().db().GetLatestByID(s.genesis.Hash)
		require.Nil(t, err)
		require.True(t, latest.Roster.ID.Equal(rosterR.ID), "roster has not been updated")
		// Get latest genesis darc and verify the 'view_change' rule is updated
		st, err := s.service().GetReadOnlyStateTrie(s.genesis.Hash)
		require.Nil(t, err)
		val, _, _, _, err := st.GetValues(s.darc.GetBaseID())
		require.Nil(t, err)
		d, err := darc.NewFromProtobuf(val)
		require.Nil(t, err)
		vcIDs := strings.Split(string(d.Rules.Get(darc.Action("invoke:view_change"))), " | ")
		require.Equal(t, len(rosterR.List), len(vcIDs))
	}

	// Make sure the latest node is correctly activated and that the
	// new conodes are done with catching up
	for _, ser := range servers {
		ctx, _ = createConfigTxWithCounter(t, testInterval, *rosterR, defaultMaxBlockSize, s, counter)
		counter++
		for i := 0; i < 2; i++ {
			_, err := ser.Service(ServiceName).(*Service).AddTransaction(&AddTxRequest{
				Version:       CurrentVersion,
				SkipchainID:   s.genesis.SkipChainID(),
				Transaction:   ctx,
				InclusionWait: 10,
			})
			if err == nil {
				break
			} else if i == 2 {
				require.Nil(t, err)
			}
			time.Sleep(testInterval)
		}
	}

	for _, node := range rosterR.List {
		log.Lvl2("Checking node", node, "has testDarc stored")
		for i := 0; i < 2; i++ {
			// Try at least during two intervals for the one block to catch up
			reply := &GetProofResponse{}
			err = leanClient.SendProtobuf(node, &GetProof{
				Version: CurrentVersion,
				ID:      s.genesis.Hash,
				Key:     testDarc.GetBaseID(),
			}, reply)
			if err == nil {
				require.Nil(t, err)
				require.True(t, reply.Proof.InclusionProof.Match(testDarc.GetBaseID()))
				break
			} else if i == 1 {
				log.Error("Couldn't get proof for darc:", err)
				t.FailNow()
			}
			time.Sleep(testInterval)
		}
	}
}

func TestService_SetConfigRosterSwitchNodes(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	_, newRoster, _ := s.local.MakeSRS(cothority.Suite, 4, ByzCoinID)

	log.Lvl1("Don't allow new nodes as new leader")
	wrongRoster := onet.NewRoster(append(newRoster.List, s.roster.List...))
	ctx, _ := createConfigTxWithCounter(t, testInterval, *wrongRoster, defaultMaxBlockSize, s, 1)
	_, err := s.services[0].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: 10,
	})
	require.NotNil(t, err)

	log.Lvl1("Allow new nodes at the end", newRoster.List)
	goodRoster := onet.NewRoster(s.roster.List)
	counter := 1
	for _, si := range newRoster.List {
		log.Lvl1("Adding", si)
		goodRoster = onet.NewRoster(append(goodRoster.List, si))
		ctx, _ = createConfigTxWithCounter(t, testInterval, *goodRoster, defaultMaxBlockSize, s, counter)
		counter++
		s.sendTxAndWait(t, ctx, 10)
	}
}

// Replaces all nodes from the previous roster with new nodes
func TestService_SetConfigRosterReplace(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	_, newRoster, _ := s.local.MakeSRS(cothority.Suite, 4, ByzCoinID)

	log.Lvl1("Replace with new roster", newRoster.List)
	goodRoster := onet.NewRoster(s.roster.List)
	counter := 1
	for _, si := range newRoster.List {
		log.Lvl1("Adding", si)
		goodRoster = onet.NewRoster(append(goodRoster.List, si))
		ctx, _ := createConfigTxWithCounter(t, testInterval, *goodRoster, defaultMaxBlockSize, s, counter)
		counter++
		cl := NewClient(s.genesis.SkipChainID(), *goodRoster)
		_, err := cl.AddTransactionAndWait(ctx, 10)
		require.Nil(t, err)

		log.Lvl1("Removing", goodRoster.List[0])
		goodRoster = onet.NewRoster(goodRoster.List[1:])
		ctx, _ = createConfigTxWithCounter(t, testInterval, *goodRoster, defaultMaxBlockSize, s, counter)
		counter++
		_, err = cl.AddTransactionAndWait(ctx, 10)
		require.Nil(t, err)
	}
}

func addDummyTxs(t *testing.T, s *ser, nbr int, perCTx int, count int) int {
	ids := []darc.Identity{s.signer.Identity()}
	for i := 0; i < nbr; i++ {
		var instrs Instructions
		for j := 0; j < perCTx; j++ {
			desc := random.Bits(256, true, random.New())
			dummyDarc := darc.NewDarc(darc.InitRules(ids, ids), desc)
			dummyDarcBuf, err := dummyDarc.ToProto()
			require.Nil(t, err)
			instr := createSpawnInstr(s.darc.GetBaseID(), ContractDarcID,
				"darc", dummyDarcBuf)
			instr.SignerCounter[0] = uint64(count)
			count++
			instrs = append(instrs, instr)
		}
		ctx, err := combineInstrsAndSign(s.signer, instrs...)
		require.Nil(t, err)

		s.sendTxAndWait(t, ctx, 10)
	}
	return count
}

func TestService_SetConfigRosterDownload(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	ids := []darc.Identity{s.signer.Identity()}
	testDarc := darc.NewDarc(darc.InitRules(ids, ids), []byte("testDarc"))
	testDarcBuf, err := testDarc.ToProto()
	require.Nil(t, err)
	instr := createSpawnInstr(s.darc.GetBaseID(), ContractDarcID, "darc", testDarcBuf)
	require.Nil(t, err)
	ctx, err := combineInstrsAndSign(s.signer, instr)
	require.Nil(t, err)
	s.sendTxAndWait(t, ctx, 10)
	// Add other transaction so we're on a new border between forward links
	ct := addDummyTxs(t, s, 4, 1, 2)

	catchupDownloadAll = 1
	_, newRoster, _ := s.local.MakeSRS(cothority.Suite, 1, ByzCoinID)

	newRoster = onet.NewRoster(append(s.roster.List, newRoster.List...))
	ctx, _ = createConfigTxWithCounter(t, testInterval, *newRoster, defaultMaxBlockSize, s, ct)
	ct++
	s.sendTxAndWait(t, ctx, 10)

	// Create a new block
	log.Lvl1("Creating two dummy blocks for the new node to catch up")
	ct = addDummyTxs(t, s, 2, 1, ct)

	log.Lvl1("And getting proof from new node that the testDarc exists")
	leanClient := onet.NewClient(cothority.Suite, ServiceName)
	reply := &GetProofResponse{}
	err = leanClient.SendProtobuf(newRoster.List[len(newRoster.List)-1], &GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.Hash,
		Key:     testDarc.GetBaseID(),
	}, reply)
	require.Nil(t, err)
	require.True(t, reply.Proof.InclusionProof.Match(testDarc.GetBaseID()))
}

func TestService_DownloadState(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	ct := addDummyTxs(t, s, 3, 3, 1)
	ct = addDummyTxs(t, s, 1, 20, ct)

	// Wrong parameters
	resp, err := s.service().DownloadState(&DownloadState{
		ByzCoinID: skipchain.SkipBlockID{},
	})
	require.NotNil(t, err)
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: skipchain.SkipBlockID{},
		Nonce:     0,
		Length:    1,
	})
	require.NotNil(t, err)
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
	})
	require.NotNil(t, err)
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     1,
	})
	require.NotNil(t, err)
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
	})
	require.NotNil(t, err)

	// Start one download and check it is aborted
	// if we start a second download.
	log.Lvl1("Check aborting of download")
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
		Length:    1,
	})
	require.Nil(t, err)
	nonce1 := resp.Nonce
	// Continue 1st download
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     nonce1,
		Length:    1,
	})
	require.Nil(t, err)
	// Start 2nd download
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
		Length:    1,
	})
	require.Nil(t, err)
	nonce2 := resp.Nonce
	require.NotEqual(t, nonce1, nonce2)
	// Now 1st download should fail
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     nonce1,
		Length:    1,
	})
	require.NotNil(t, err)
	// And 2nd download should still continue
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     nonce2,
		Length:    1,
	})
	require.Nil(t, err)

	// Start downloading
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
		Length:    10,
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 10, len(resp.KeyValues))

	// Start a new download and go till the end
	length := 0
	var nonce uint64
	for {
		resp, err = s.service().DownloadState(&DownloadState{
			ByzCoinID: s.genesis.SkipChainID(),
			Nonce:     nonce,
			Length:    10,
		})
		require.Nil(t, err)
		if len(resp.KeyValues) == 0 {
			break
		}
		length += len(resp.KeyValues)
		nonce = resp.Nonce
	}
	// As we copy the whole db, also the interior nodes
	// are copied, so we cannot know in advance how many
	// entries we copy...
	require.True(t, length > 40)

	time.Sleep(time.Second)
	// Try to re-create the trie on a new service -
	// do it twice
	for i := 0; i < 2; i++ {
		servers, _, _ := s.local.MakeSRS(cothority.Suite, 1, ByzCoinID)
		services := s.local.GetServices(servers, ByzCoinID)
		service := services[0].(*Service)
		err := service.downloadDB(s.genesis)
		require.Nil(t, err)
		st, err := service.getStateTrie(s.genesis.Hash)
		require.Nil(t, err)
		val, _, _, _, err := st.GetValues(make([]byte, 32))
		require.Nil(t, err)
		require.True(t, len(val) > 0)
		configCopy := ChainConfig{}
		err = protobuf.DecodeWithConstructors(val, &configCopy, network.DefaultConstructors(cothority.Suite))
	}
}

func TestService_SetBadConfig(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// send in a bad new block size
	ctx, badConfig := createBadConfigTx(t, s, false, true)
	s.sendTx(t, ctx)

	// wait for a change, which should not happen
	for i := 0; i < 5; i++ {
		time.Sleep(s.interval)
		config, err := s.service().LoadConfig(s.genesis.SkipChainID())
		require.NoError(t, err)

		if badConfig.Roster.List[0].Equal(config.Roster.List[0]) {
			require.Fail(t, "found a bad config")
		}
	}

	// send in a bad new interval
	ctx, badConfig = createBadConfigTx(t, s, true, false)
	s.sendTx(t, ctx)

	// wait for a change, which should not happen
	for i := 0; i < 5; i++ {
		time.Sleep(s.interval)
		config, err := s.service().LoadConfig(s.genesis.SkipChainID())
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
	testViewChange(t, 4, 1, 4*time.Second)
}

func TestService_ViewChange2(t *testing.T) {
	if testing.Short() {
		t.Skip("doesn't work on travis correctly due to byzcoinx timeout issue, see #1428")
	}
	testViewChange(t, 7, 2, 4*time.Second)
}

func testViewChange(t *testing.T, nHosts, nFailures int, interval time.Duration) {
	rw := rotationWindow
	defer func() {
		rotationWindow = rw
	}()
	rotationWindow = 3
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
	for i := 0; i < nFailures; i++ {
		time.Sleep(time.Duration(math.Pow(2, float64(i))) * s.interval * rotationWindow)
	}
	time.Sleep(2 * s.interval)
	config, err := s.services[nFailures].LoadConfig(s.genesis.SkipChainID())
	require.NoError(t, err)
	log.Lvl2("Verifying roster", config.Roster.List)
	require.True(t, config.Roster.List[0].Equal(s.services[nFailures].ServerIdentity()))

	// check that the leader is updated for all nodes
	for _, service := range s.services[nFailures:] {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(s.genesis.SkipChainID())
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
	require.True(t, pr.InclusionProof.Match(tx1.Instructions[0].InstanceID.Slice()))

	// The transaction should also be stored on followers
	for i := nFailures + 1; i < nHosts; i++ {
		pr = s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID.Slice(), i)
		require.True(t, pr.InclusionProof.Match(tx1.Instructions[0].InstanceID.Slice()))
	}

	// We need to bring the failed (the first nFailures) nodes back up and
	// check that they can synchronise to the latest state.
	for i := 0; i < nFailures; i++ {
		log.Lvl1("starting node at index", i)
		s.hosts[i].Unpause()
		require.NoError(t, s.services[i].startAllChains())
	}
	for i := 0; i < nFailures; i++ {
		pr = s.waitProofWithIdx(t, tx1.Instructions[0].InstanceID.Slice(), i)
		require.True(t, pr.InclusionProof.Match(tx1.Instructions[0].InstanceID.Slice()))
	}

	log.Lvl1("Sending 1st tx")
	tx1, err = createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, 1)
	require.NoError(t, err)
	s.sendTxToAndWait(t, tx1, nFailures, 10)
	log.Lvl1("Sending 2nd tx")
	tx1, err = createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, 2)
	require.NoError(t, err)
	s.sendTxToAndWait(t, tx1, nFailures, 10)
	log.Lvl1("Sent two tx")
}

func TestService_DarcToSc(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	darcID := s.darc.GetBaseID()
	scID := s.genesis.SkipChainID()

	// check that the mapping is correct
	for _, service := range s.services {
		require.True(t, service.darcToSc[string(darcID)].Equal(scID))
	}

	// remove the mapping and then load it again
	log.Lvl1("Reloading all services")
	for _, service := range s.services {
		service.darcToSc = make(map[string]skipchain.SkipBlockID)
		service.TestClose()
		require.NoError(t, service.startAllChains())
	}

	// check that the mapping is still correct
	log.Lvl1("Verifying mapping is still correct")
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
	contractID := "stateChangeCacheTest"
	var ctr int
	contract := func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
		ctr++
		return []StateChange{}, []Coin{}, nil
	}
	s.service().registerContract(contractID, adaptor(contract))

	scID := s.genesis.SkipChainID()
	st, err := s.service().getStateTrie(scID)
	require.NoError(t, err)
	sst := st.MakeStagingStateTrie()
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), contractID, []byte{}, s.signer, 1)
	require.Nil(t, err)

	// Add a second tx that is invalid because it is for an unknown contract.
	log.Lvl1("Calling invalid invoke on contract")
	tx2, err := createOneClientTxWithCounter(s.darc.GetBaseID(), contractID+"x", []byte{}, s.signer, 2)
	require.Nil(t, err)

	txs := NewTxResults(tx1, tx2)
	require.NoError(t, err)
	root, txOut, states, _ := s.service().createStateChanges(sst, scID, txs, noTimeout)
	require.Equal(t, 2, len(txOut))
	require.Equal(t, 1, ctr)
	// we expect one state change to increment the signature counter
	require.Equal(t, 1, len(states))
	require.Equal(t, []byte{}, states[0].ContractID)
	require.Equal(t, []byte{}, []byte(states[0].DarcID))

	// If we call createStateChanges on the new txOut (as it will happen in production
	// when the tx set is reduced by the selection step, and then txOut are sent to
	// createStateChanges when making the block), then it should load it from the
	// cache, which means that ctr is still one (we do not call the
	// contract twice).
	root1, txOut1, states1, _ := s.service().createStateChanges(sst, scID, txOut, noTimeout)
	require.Equal(t, 1, ctr)
	require.Equal(t, root, root1)
	require.Equal(t, txOut, txOut1)
	require.Equal(t, states, states1)

	// If we remove the cache, then we expect the contract to be called
	// again, i.e., ctr == 2.
	s.service().stateChangeCache = newStateChangeCache()
	require.NoError(t, err)
	root2, txOut2, states2, _ := s.service().createStateChanges(sst, scID, txs, noTimeout)
	require.Equal(t, root, root2)
	require.Equal(t, txOut, txOut2)
	require.Equal(t, states, states2)
	require.Equal(t, 2, ctr)
}

// Check that we got no error from an existing state trie
func TestService_UpdateTrieCallback(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// already announced but it should exit silently
	// as the trie index is different
	err := s.service().updateTrieCallback(s.genesis.SkipChainID())
	require.Nil(t, err)
}

// This tests that the state change storage will actually
// store them and increase the versions accordingly over
// several transactions and instructions
func TestService_StateChangeStorage(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	signerIID := NewInstanceID(publicVersionKey(s.signer.Identity().String()))

	n := 2
	iid := genID()
	contractID := "stateChangeCacheTest"
	contract := func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
		// Check the version is correctly increased for multiple state changes
		var scs []StateChange
		if _, _, _, _, err := cdb.GetValues(iid.Slice()); err == errKeyNotSet {
			scs = []StateChange{{
				StateAction: Create,
				InstanceID:  iid[:],
			}}
		}
		scs = append(scs, StateChange{
			StateAction: Update,
			InstanceID:  iid[:],
		})
		scs = append(scs, StateChange{ // this one should not increase the version of the previous two
			StateAction: Create,
			InstanceID:  inst.DeriveID("").Slice(),
		})
		return scs, []Coin{}, nil
	}
	for _, s := range s.hosts {
		RegisterContract(s, contractID, adaptor(contract))
	}

	for i := 0; i < n; i++ {
		tx, err := createClientTxWithTwoInstrWithCounter(s.darc.GetBaseID(), contractID, []byte{}, s.signer, uint64(i*2+1))
		require.Nil(t, err)

		// Queue all transactions, except for the last one
		wait := 0
		if i == n-1 {
			wait = n
		}
		_, err = s.service().AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   s.genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: wait,
		})
		require.Nil(t, err)
	}

	scID := s.genesis.SkipChainID()
	for _, service := range s.services {
		log.Lvl1("Checking service", service.ServerIdentity())
		// Waiting for other nodes to include the latest
		// statechanges.
		for i := 0; i < 10; i++ {
			log.Lvl2("Checking signer")
			res, err := service.GetAllInstanceVersion(&GetAllInstanceVersion{
				SkipChainID: scID,
				InstanceID:  signerIID,
			})
			require.Nil(t, err)
			if len(res.StateChanges) == n*2 {
				break
			}
			// Even if the leader got the block, the other nodes still need time to
			// apply the block
			time.Sleep(testInterval)
		}

		res, err := service.GetAllInstanceVersion(&GetAllInstanceVersion{
			SkipChainID: scID,
			InstanceID:  iid,
		})
		require.Nil(t, err)
		require.Equal(t, 2*n+1, len(res.StateChanges))

		log.Lvlf1("Getting versions of iid %x and signer %x", iid[:], signerIID[:])
		for i := 0; i < n*2; i++ {
			log.Lvlf1("Getting version %d", i)
			sc, err := service.GetInstanceVersion(&GetInstanceVersion{
				SkipChainID: scID,
				InstanceID:  iid,
				Version:     uint64(i),
			})
			require.Nil(t, err)
			require.Equal(t, uint64(i), sc.StateChange.Version)
			require.Equal(t, uint64(i), res.StateChanges[i].StateChange.Version)

			res, err := service.CheckStateChangeValidity(&CheckStateChangeValidity{
				SkipChainID: scID,
				InstanceID:  iid,
				Version:     uint64(i),
			})
			require.Nil(t, err)

			sb, err := service.skService().GetSingleBlock(&skipchain.GetSingleBlock{ID: res.BlockID})
			require.Nil(t, err)
			var header DataHeader
			err = protobuf.DecodeWithConstructors(sb.Data, &header, network.DefaultConstructors(cothority.Suite))
			require.Nil(t, err)
			require.Equal(t, StateChanges(res.StateChanges).Hash(), header.StateChangesHash)
		}

		log.Lvl1("Checking last version of iid")
		sc, err := service.GetLastInstanceVersion(&GetLastInstanceVersion{
			SkipChainID: scID,
			InstanceID:  iid,
		})
		require.Nil(t, err, "iid key not found")
		require.Equal(t, uint64(n*2), sc.StateChange.Version)

		log.Lvl1("Checking last version of signer")
		sc, err = service.GetLastInstanceVersion(&GetLastInstanceVersion{
			SkipChainID: scID,
			InstanceID:  signerIID,
		})
		require.Nil(t, err, "signer key not found")
		require.Equal(t, uint64(n*2), sc.StateChange.Version)
	}
}

// This tests that the service will restore the state changes
// after a (re)boot and catch up potential new blocks
func TestService_StateChangeCatchUp(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	n := 5
	contractID := "stateChangeCacheTest"
	contract := func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
		// Check the state trie is created from the known global state
		iid := inst.Hash()
		if !bytes.Equal(inst.InstanceID.Slice(), s.darc.GetBaseID()) {
			iid = inst.InstanceID.Slice()
		}
		_, ver, _, _, err := cdb.GetValues(iid)
		sc1 := StateChange{
			StateAction: Update,
			InstanceID:  iid,
			ContractID:  []byte(contractID),
			Version:     ver + 1,
		}
		if err != nil {
			sc1.StateAction = Create
		}
		return []StateChange{sc1}, []Coin{}, nil
	}
	for _, s := range s.hosts {
		RegisterContract(s, contractID, adaptorNoVerify(contract))
	}

	createTx := func(iid []byte, counter uint64, wait int) *Instruction {
		instr := Instruction{
			InstanceID:    NewInstanceID(iid),
			Spawn:         &Spawn{ContractID: contractID},
			SignerCounter: []uint64{counter},
		}
		tx := ClientTransaction{Instructions: Instructions{instr}}
		err := tx.Instructions[0].SignWith(tx.Instructions.Hash(), s.signer)
		require.Nil(t, err)

		_, err = s.service().AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   s.genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: wait,
		})
		require.Nil(t, err)

		return &instr
	}

	instr := createTx(s.darc.GetBaseID(), uint64(1), 1)

	for i := 0; i < n-1; i++ {
		// add transactions that must be recreated
		createTx(instr.Hash(), uint64(i+1), 0)
	}
	createTx(instr.Hash(), uint64(n), 2)

	// Remove some entries to check it will recreate them
	err := s.service().stateChangeStorage.db.Update(func(tx *bolt.Tx) error {
		b := s.service().stateChangeStorage.getBucket(tx, s.genesis.SkipChainID())
		if b == nil {
			return errors.New("missing bucket")
		}

		c := b.Cursor()
		// Remove entries associated with the second block
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if k[len(k)-1] == byte(2) {
				err := c.Delete()
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	require.Nil(t, err)

	scs, err := s.service().stateChangeStorage.getAll(instr.Hash(), s.genesis.SkipChainID())
	require.Nil(t, err)
	require.Equal(t, 1, len(scs))

	s.service().trySyncAll()

	scs, err = s.service().stateChangeStorage.getAll(instr.Hash(), s.genesis.SkipChainID())
	require.Nil(t, err)
	require.Equal(t, n+1, len(scs))
	require.Equal(t, uint64(n), scs[n].StateChange.Version)

	counterID := publicVersionKey(s.signer.Identity().String())
	sc, ok, err := s.service().stateChangeStorage.getLast(counterID, s.genesis.SkipChainID())
	require.Nil(t, err)
	require.True(t, ok)
	require.Equal(t, uint64(n+1), sc.StateChange.Version)
}

func createBadConfigTx(t *testing.T, s *ser, intervalBad, szBad bool) (ClientTransaction, ChainConfig) {
	switch {
	case intervalBad:
		return createConfigTxWithCounter(t, -1, *s.roster.RandomSubset(s.services[1].ServerIdentity(), 2), defaultMaxBlockSize, s, 1)
	case szBad:
		return createConfigTxWithCounter(t, 420*time.Millisecond, *s.roster.RandomSubset(s.services[1].ServerIdentity(), 2), 30*1e6, s, 1)
	default:
		return createConfigTxWithCounter(t, 420*time.Millisecond, *s.roster, 424242, s, 1)
	}
}

func createConfigTxWithCounter(t *testing.T, interval time.Duration, roster onet.Roster, size int, s *ser, counter int) (ClientTransaction, ChainConfig) {
	config := ChainConfig{
		BlockInterval: interval,
		Roster:        roster,
		MaxBlockSize:  size,
	}
	configBuf, err := protobuf.Encode(&config)
	require.NoError(t, err)

	instr := Instruction{
		InstanceID: NewInstanceID(nil),
		Invoke: &Invoke{
			Command: "update_config",
			Args: []Argument{{
				Name:  "config",
				Value: configBuf,
			}},
		},
		SignerCounter: []uint64{uint64(counter)},
	}
	ctx, err := combineInstrsAndSign(s.signer, instr)

	require.NoError(t, err)
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
		InstanceID:    NewInstanceID(d2.GetBaseID()),
		Invoke:        &invoke,
		SignerCounter: []uint64{1},
	}
	ctx, err := combineInstrsAndSign(signer, instr)
	require.NoError(t, err)
	return ctx
}

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
	genesis  *skipchain.SkipBlock
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
		resp, err := s.services[idx].GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     key,
			ID:      s.genesis.SkipChainID(),
		})
		if err == nil {
			pr = resp.Proof
			if pr.InclusionProof.Match(key) {
				ok = true
				break
			}
		}

		// wait for the block to be processed
		time.Sleep(2 * s.interval)
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
		SkipchainID: s.genesis.SkipChainID(),
		Transaction: ctx,
	})
	require.Nil(t, err)
}

func (s *ser) sendTxAndWait(t *testing.T, ctx ClientTransaction, wait int) {
	s.sendTxToAndWait(t, ctx, 0, wait)
}

func (s *ser) sendTxToAndWait(t *testing.T, ctx ClientTransaction, idx int, wait int) {
	_, err := s.services[idx].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: wait,
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
			ID:      s.genesis.SkipChainID(),
		})
		require.Nil(t, err)
		pr = &resp.Proof
		_, v0, _, _, err := pr.KeyValue()
		require.Nil(t, err)
		d, err := darc.NewFromProtobuf(v0)
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
	for _, sv := range s.local.GetServices(s.hosts, ByzCoinID) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}
	registerDummy(s.hosts)

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster,
		[]string{"spawn:dummy", "spawn:invalid", "spawn:panic", "spawn:darc", "invoke:update_config", "spawn:slow", "spawn:stateChangeCacheTest", "delete"}, s.signer.Identity())
	require.Nil(t, err)
	s.darc = &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = interval
	s.interval = genesisMsg.BlockInterval

	for i := 0; i < step; i++ {
		switch i {
		case 0:
			resp, err := s.service().CreateGenesisBlock(genesisMsg)
			require.Nil(t, err)
			s.genesis = resp.Skipblock
		case 1:
			tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
			require.Nil(t, err)
			s.tx = tx
			_, err = s.service().AddTransaction(&AddTxRequest{
				Version:       CurrentVersion,
				SkipchainID:   s.genesis.SkipChainID(),
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

type contractAdaptor struct {
	BasicContract
	cb func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error)
}

func (ca *contractAdaptor) Spawn(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return ca.cb(cdb, inst, c)
}

func (ca *contractAdaptor) Invoke(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return ca.cb(cdb, inst, c)
}

func (ca *contractAdaptor) Delete(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return ca.cb(cdb, inst, c)
}

type contractAdaptorNV struct {
	BasicContract
	cb func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error)
}

func (ca *contractAdaptorNV) VerifyInstruction(cdb ReadOnlyStateTrie, inst Instruction, msg []byte) error {
	// Always verifies the instruction as "ok".
	return nil
}

func (ca *contractAdaptorNV) Spawn(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return ca.cb(cdb, inst, c)
}

func (ca *contractAdaptorNV) Invoke(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return ca.cb(cdb, inst, c)
}

func (ca *contractAdaptorNV) Delete(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return ca.cb(cdb, inst, c)
}

// adaptor turns an old-style contract callback into a new-style contract.
func adaptor(cb func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error)) func([]byte) (Contract, error) {
	return func([]byte) (Contract, error) {
		return &contractAdaptor{cb: cb}, nil
	}
}

// adaptorNoVerify turns an old-style contract callback into a new-style contract
// but uses a stub verifier (for use when testing createStateChanges, where Darcs
// are not in place)
func adaptorNoVerify(cb func(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error)) func([]byte) (Contract, error) {
	return func([]byte) (Contract, error) {
		return &contractAdaptorNV{cb: cb}, nil
	}
}

func invalidContractFunc(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	return nil, nil, errors.New("this invalid contract always returns an error")
}

func panicContractFunc(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	panic("this contract panics")
}

func dummyContractFunc(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	_, _, _, darcID, err := cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	switch inst.GetType() {
	case SpawnType:
		if len(inst.Spawn.Args.Search("data")) == 32 {
			return []StateChange{
				NewStateChange(Create, NewInstanceID(inst.Spawn.Args.Search("data")), inst.Spawn.ContractID,
					[]byte{}, darcID),
			}, nil, nil
		}
		return []StateChange{
			NewStateChange(Create, NewInstanceID(inst.Hash()), inst.Spawn.ContractID, inst.Spawn.Args[0].Value, darcID),
		}, nil, nil
	case InvokeType:
		return []StateChange{
			NewStateChange(Update, inst.InstanceID, dummyContract, inst.Invoke.Args[0].Value, darcID),
		}, nil, nil
	case DeleteType:
		return []StateChange{
			NewStateChange(Remove, inst.InstanceID, "", nil, darcID),
		}, nil, nil
	default:
		panic("should not get here")
	}
}

func slowContractFunc(cdb ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	// This has to sleep for less than testInterval / 2 or else it will
	// block the system from processing txs. See #1359.
	time.Sleep(testInterval / 5)
	return dummyContractFunc(cdb, inst, c)
}

func registerDummy(servers []*onet.Server) {
	// For testing - there must be a better way to do that. But putting
	// services []skipchain.Service in the method signature doesn't work :(
	for _, s := range servers {
		err := RegisterContract(s, dummyContract, adaptor(dummyContractFunc))
		log.ErrFatal(err)
		err = RegisterContract(s, slowContract, adaptor(slowContractFunc))
		log.ErrFatal(err)
		err = RegisterContract(s, invalidContract, adaptor(invalidContractFunc))
		log.ErrFatal(err)
	}
}

func genID() (i InstanceID) {
	random.Bytes(i[:], random.New())
	return i
}

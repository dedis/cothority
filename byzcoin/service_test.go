package byzcoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/xerrors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/sign/eddsa"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

var tSuite = suites.MustFind("Ed25519")
var testInterval = 500 * time.Millisecond

// use this value as a rotation window to make it impossible to trigger a view change
var disableViewChange = time.Duration(9999)

const dummyContract = "dummy"
const slowContract = "slow"
const panicContract = "panic"
const invalidContract = "invalid"
const versionContract = "testVersionContract"
const stateChangeCacheContract = "stateChangeCacheTest"

func TestMain(m *testing.M) {
	log.SetShowTime(true)
	log.MainTest(m)
}

func TestService_GetAllByzCoinIDs(t *testing.T) {
	s := newSerN(t, 1, testInterval, 4, disableViewChange)
	defer s.local.CloseAll()

	service := s.services[0]

	resp, err := service.GetAllByzCoinIDs(&GetAllByzCoinIDsRequest{})
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.IDs))

	nb := skipchain.NewSkipBlock()
	nb.Roster = s.roster
	nb.MaximumHeight = 1
	nb.BaseHeight = 1
	_, err = service.skService().StoreSkipBlockInternal(&skipchain.StoreSkipBlock{NewBlock: nb})
	require.NoError(t, err)

	resp, err = service.GetAllByzCoinIDs(&GetAllByzCoinIDsRequest{})
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.IDs))
}

func TestService_CreateGenesisBlock(t *testing.T) {
	s := newSerN(t, 0, testInterval, 4, disableViewChange)
	defer s.local.CloseAll()

	service := s.services[1]

	// invalid version, missing transaction
	_, err := service.CreateGenesisBlock(&CreateGenesisBlock{
		Version: 0,
		Roster:  *s.roster,
	})
	require.Error(t, err)

	// invalid: max block too small, big
	_, err = service.CreateGenesisBlock(&CreateGenesisBlock{
		Version:      0,
		Roster:       *s.roster,
		MaxBlockSize: 3000,
	})
	require.Error(t, err)
	_, err = service.CreateGenesisBlock(&CreateGenesisBlock{
		Version:      0,
		Roster:       *s.roster,
		MaxBlockSize: 30 * 1e6,
	})
	require.Error(t, err)

	// invalid darc
	_, err = service.CreateGenesisBlock(&CreateGenesisBlock{
		Version:     CurrentVersion,
		Roster:      *s.roster,
		GenesisDarc: darc.Darc{},
	})
	require.Error(t, err)

	// create valid darc
	signer := darc.NewSignerEd25519(nil, nil)
	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster, []string{"spawn:dummy"}, signer.Identity())
	require.NoError(t, err)
	genesisMsg.BlockInterval = 100 * time.Millisecond
	genesisMsg.MaxBlockSize = 1 * 1e6

	// finally passing
	resp, err := service.CreateGenesisBlock(genesisMsg)
	require.NoError(t, err)
	assert.Equal(t, CurrentVersion, resp.Version)
	assert.NotNil(t, resp.Skipblock)

	proof, err := service.GetProof(&GetProof{
		Version: CurrentVersion,
		Key:     genesisMsg.GenesisDarc.GetID(),
		ID:      resp.Skipblock.SkipChainID(),
	})
	require.NoError(t, err)
	require.Nil(t, proof.Proof.Verify(resp.Skipblock.SkipChainID()))
	k, _, _, _, err := proof.Proof.KeyValue()
	require.NoError(t, err)
	require.EqualValues(t, genesisMsg.GenesisDarc.GetID(), k)

	interval, maxsz, err := service.LoadBlockInfo(resp.Skipblock.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, interval, genesisMsg.BlockInterval)
	require.Equal(t, maxsz, genesisMsg.MaxBlockSize)
}

func TestService_AddTransaction(t *testing.T) {
	testAddTransaction(t, testInterval, 0, false)
}

func TestService_AddTransaction_ToFollower(t *testing.T) {
	testAddTransaction(t, testInterval, 1, false)
}

func TestService_AddTransaction_WithFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Keep edge cases for Jenkins")
	}
	testAddTransaction(t, testInterval, 0, true)
}

func TestService_AddTransaction_WithFailure_OnFollower(t *testing.T) {
	if testing.Short() {
		t.Skip("Keep edge cases for Jenkins")
	}
	testAddTransaction(t, testInterval, 1, true)
}

func transactionOK(t *testing.T, resp *AddTxResponse, err error) {
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Error)
}

func testAddTransaction(t *testing.T, blockInterval time.Duration, sendToIdx int, failure bool) {
	var s *ser
	if failure {
		s = newSerN(t, 1, blockInterval, 4, disableViewChange)
		for _, service := range s.services {
			service.SetPropagationTimeout(blockInterval * 2)
		}
	} else {
		s = newSer(t, 1, testInterval)
	}
	defer s.local.CloseAll()

	// wrong version
	_, err := s.service().AddTransaction(&AddTxRequest{
		Version: CurrentVersion + 1,
	})
	require.Error(t, err)

	// missing skipchain
	_, err = s.service().AddTransaction(&AddTxRequest{
		Version: CurrentVersion,
	})
	require.Error(t, err)

	// missing transaction
	_, err = s.service().AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.genesis.SkipChainID(),
	})
	require.Error(t, err)

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
	require.NoError(t, err)
	akvresp, err := s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 10,
	})
	transactionOK(t, akvresp, err)
	require.Equal(t, CurrentVersion, akvresp.Version)
	require.NotNil(t, akvresp.Proof)
	require.NoError(t, akvresp.Proof.VerifyFromBlock(s.genesis))

	// add the second tx
	log.Lvl1("adding the second tx")
	value2 := []byte("value2")
	tx2, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, value2, s.signer, 2)
	require.NoError(t, err)
	akvresp, err = s.services[sendToIdx].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 10,
	})
	transactionOK(t, akvresp, err)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// try to read the transaction back again
	log.Lvl1("reading the transactions back")
	txs := []ClientTransaction{tx1, tx2}
	for i := 0; i < 2; i++ {
		if i == 1 {
			// Now read the key/values from a new service
			log.Lvl1("Recreate services and fetch keys again")
			s.service().TestClose()
			require.NoError(t, s.service().TestRestart())
		}
		for _, tx := range txs {
			pr := s.waitProofWithIdx(t, tx.Instructions[0].Hash(), 0)
			require.Nil(t, pr.Verify(s.genesis.SkipChainID()))
			_, v0, _, _, err := pr.KeyValue()
			require.NoError(t, err)
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
		require.NoError(t, s.services[len(s.hosts)-1].TestRestart())

		for _, tx := range txs {
			pr := s.waitProofWithIdx(t, tx.Instructions[0].Hash(), len(s.hosts)-1)
			require.Nil(t, pr.Verify(s.genesis.SkipChainID()))
			_, v0, _, _, err := pr.KeyValue()
			require.NoError(t, err)
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
		pr, k, resp, err, err2 := sendTransaction(t, s, len(s.hosts)-1, dummyContract, 10)
		transactionOK(t, resp, err)
		require.NoError(t, err2)
		require.True(t, pr.InclusionProof.Match(k))

		log.Lvl1("done")
		// Wait for tasks to finish.
		time.Sleep(blockInterval)
	}
}

func TestService_AddTransaction_WrongNode(t *testing.T) {
	defer log.SetShowTime(log.ShowTime())
	log.SetShowTime(true)
	s := newSerN(t, 1, testInterval, 4, disableViewChange)
	defer s.local.CloseAll()

	outsideServer := s.local.GenServers(1)[0]
	outside := outsideServer.Service(ServiceName).(*Service)
	registerDummy(t, []*onet.Server{outsideServer})

	// add the first tx to outside server
	log.Lvl1("adding the first tx - this should fail")
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, 1)
	require.NoError(t, err)
	atx := &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 5,
	}
	_, err = outside.AddTransaction(atx)
	require.Error(t, err)

	// Adding outside to roster
	log.Lvl1("Adding new node to the roster")
	rosterR := onet.NewRoster(append(s.roster.List, outside.ServerIdentity()))
	ctx, _ := createConfigTxWithCounter(t, testInterval, *rosterR, defaultMaxBlockSize, s, 1)
	s.sendTxAndWait(t, ctx, 10)

	// force the synchronization as the new node needs to get the
	// propagation to know about the skipchain but we're not testing that
	// here
	proof, err := s.service().db().GetProof(s.genesis.Hash)
	require.NoError(t, err)
	_, err = outside.db().StoreBlocks(proof)
	require.NoError(t, err)

	log.Lvl1("adding tx to now included node")
	atx.Transaction, err = createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, 2)
	require.NoError(t, err)
	resp, err := outside.AddTransaction(atx)
	transactionOK(t, resp, err)
}

// Tests what happens if a transaction with two instructions is sent: one valid
// and one invalid instruction.
func TestService_AddTransaction_ValidInvalid(t *testing.T) {
	defer log.SetShowTime(log.ShowTime())
	log.SetShowTime(true)
	s := newSerN(t, 1, testInterval, 4, disableViewChange)
	defer s.local.CloseAll()

	// add the first tx to create the instance
	log.Lvl1("Adding the first tx")
	dcID := random.Bits(256, false, random.New())
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, dcID, s.signer, 1)
	require.NoError(t, err)
	atx := &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 5,
	}
	resp, err := s.service().AddTransaction(atx)
	transactionOK(t, resp, err)

	// add a second tx that holds two instructions: one valid and one invalid (creates the same contract)
	log.Lvl1("Adding the second tx")
	instr1 := createInvokeInstr(NewInstanceID(dcID), ContractDarcID, cmdDarcEvolve, "data", dcID)
	instr1.SignerIdentities = []darc.Identity{s.signer.Identity()}
	instr1.SignerCounter = []uint64{2}
	instr2 := createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", dcID)
	instr2.SignerIdentities = []darc.Identity{s.signer.Identity()}
	instr2.SignerCounter = []uint64{3}
	tx2 := NewClientTransaction(CurrentVersion, instr1, instr2)
	h := tx2.Instructions.Hash()
	for i := range tx2.Instructions {
		err := tx2.Instructions[i].SignWith(h, s.signer)
		require.NoError(t, err)
	}
	atx = &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 5,
	}
	resp, err = s.service().AddTransaction(atx)
	require.NoError(t, err)
	require.Contains(t, resp.Error, "contract darc tried to create existing instanceID")

	// add a third tx that holds two valid instructions
	log.Lvl1("Adding a third, valid tx")
	instr1 = createInvokeInstr(NewInstanceID(dcID), ContractDarcID, cmdDarcEvolve, "data", dcID)
	instr1.SignerCounter = []uint64{2}
	instr1.SignerIdentities = []darc.Identity{s.signer.Identity()}
	dcID2 := random.Bits(256, true, random.New())
	instr2 = createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", dcID2)
	instr2.SignerCounter = []uint64{3}
	instr2.SignerIdentities = []darc.Identity{s.signer.Identity()}
	tx3 := NewClientTransaction(CurrentVersion, instr1, instr2)
	tx3.SignWith(s.signer)
	atx = &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx3,
		InclusionWait: 5,
	}
	resp, err = s.service().AddTransaction(atx)
	transactionOK(t, resp, err)
}

// Sends the same transaction to two different nodes and makes sure that it shows up only once in a
// block.
func TestService_AddTransaction_Parallel(t *testing.T) {
	defer log.SetShowTime(log.ShowTime())
	log.SetShowTime(true)
	s := newSerN(t, 1, testInterval, 4, disableViewChange)
	defer s.local.CloseAll()

	// add the first tx to create the instance
	log.Lvl1("Adding a tx twice")
	dcID := random.Bits(256, false, random.New())
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, dcID, s.signer, 1)
	require.NoError(t, err)
	atx := &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 0,
	}

	// This is somewhat racy, as we could just fall between the creation of a block. So fingers crossed that
	// both transactions are sent to the same block.
	resp, err := s.service().AddTransaction(atx)
	transactionOK(t, resp, err)
	atx.InclusionWait = 5
	resp, err = s.services[1].AddTransaction(atx)
	transactionOK(t, resp, err)

	// Get latest block and count the number of transactions
	proof, err := s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		Key:     tx1.Instructions[0].DeriveID("").Slice(),
		ID:      s.genesis.Hash,
	})
	require.NoError(t, err)
	var payload DataBody
	require.NoError(t, protobuf.Decode(proof.Proof.Latest.Payload, &payload))
	require.Equal(t, 1, len(payload.TxResults))

	// Test if the same transaction is still rejected a block later - it should be rejected.
	log.Lvl1("Adding same tx again")
	atx.InclusionWait = 0
	resp, err = s.services[1].AddTransaction(atx)
	transactionOK(t, resp, err)

	log.Lvl1("Adding another transaction to create block")
	dcID = random.Bits(256, false, random.New())
	atx.Transaction, err = createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, dcID, s.signer, 2)
	require.NoError(t, err)
	atx.InclusionWait = 5
	resp, err = s.services[1].AddTransaction(atx)
	transactionOK(t, resp, err)

	// Get latest block and make sure that it didn't get added
	proof, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		Key:     tx1.Instructions[0].DeriveID("").Slice(),
		ID:      s.genesis.Hash,
	})
	require.NoError(t, err)
	// No idea why the payload needs to be reset here - probably an error in the protobuf library.
	payload = DataBody{}
	require.NoError(t, protobuf.Decode(proof.Proof.Latest.Payload, &payload))
	require.Equal(t, 1, len(payload.TxResults))
}

// Test that a contract have access to the ByzCoin protocol version.
func TestService_AddTransactionVersion(t *testing.T) {
	s := newSerWithVersion(t, 1, testInterval, 4, disableViewChange, 0)
	defer s.local.CloseAll()

	// Send the first tx with a version 0 of the ByzCoin protocol. The contract
	// checks that the value is equal to the version.
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), versionContract, []byte{0}, s.signer, 1)
	require.NoError(t, err)
	atx := &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 10,
	}
	_, err = s.service().AddTransaction(atx)
	require.NoError(t, err)

	// Upgrade the chain with a special block.
	_, err = s.service().createUpgradeVersionBlock(s.genesis.Hash, 1)
	require.NoError(t, err)

	// Send another tx this time for the version 1 of the ByzCoin protocol.
	tx2, err := createOneClientTxWithCounter(s.darc.GetBaseID(), versionContract, []byte{1}, s.signer, 2)
	require.NoError(t, err)
	atx = &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 10,
	}
	_, err = s.service().AddTransaction(atx)
	require.NoError(t, err)

	// This one will fail as the version must be 1.
	tx3, err := createOneClientTxWithCounter(s.darc.GetBaseID(), versionContract, []byte{0}, s.signer, 3)
	require.NoError(t, err)
	atx = &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx3,
		InclusionWait: 10,
	}
	reply, err := s.service().AddTransaction(atx)
	require.NoError(t, err)
	require.Contains(t, reply.Error, "wrong byzcoin version")
}

func TestService_AutomaticVersionUpgrade(t *testing.T) {
	// Creates a chain starting with version 0.
	s := newSerWithVersion(t, 1, testInterval, 4, disableViewChange, 0)
	defer s.local.CloseAll()

	closing := make(chan bool)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(closeChan chan bool) {
		defer wg.Done()

		c := uint64(1)
		shutdown := false
		wait := 0
		for !shutdown {
			select {
			case <-closing:
				shutdown = true
				wait = 10
			default:
			}

			tx, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, c)
			require.NoError(t, err)
			atx := &AddTxRequest{
				Version:       CurrentVersion,
				SkipchainID:   s.genesis.SkipChainID(),
				Transaction:   tx,
				InclusionWait: wait,
			}
			_, err = s.service().AddTransaction(atx)
			require.NoError(t, err)

			c++
			time.Sleep(50 * time.Millisecond)
		}
	}(closing)

	time.Sleep(testInterval)

	// Simulate an upgrade of the conodes.
	for _, srv := range s.services {
		srv.defaultVersionLock.Lock()
		srv.defaultVersion = CurrentVersion
		srv.defaultVersionLock.Unlock()
	}

	for i := 0; i < 10; i++ {
		time.Sleep(testInterval)

		proof, err := s.service().GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     NewInstanceID([]byte{}).Slice(),
			ID:      s.genesis.Hash,
		})
		require.NoError(t, err)

		header, err := decodeBlockHeader(&proof.Proof.Latest)
		require.NoError(t, err)
		if header.Version == CurrentVersion {
			close(closing)
			wg.Wait()
			return
		}
	}

	close(closing)
	wg.Wait()
	t.Fail()
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
		require.NoError(t, err)
		if rep.Proof.InclusionProof.Match(serKey) {
			break
		}
	}
	require.NotEqual(t, 10, i, "didn't get proof in time")
	key, v0, _, _, err := rep.Proof.KeyValue()
	require.Equal(t, key, serKey)
	require.NoError(t, err)
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

		// In this simulation, we can make a signature the simple way:
		// eddsa.Sign With auth proxies which are using DSS, the client
		// will contact proxies to get signatures, then interpolate
		// them into the final signature.
		return ed.Sign(msg2)
	}

	// now set the signer with the correct callback
	signer = darc.NewSignerProxy(email, ed.Public, ga)
	ctx := NewClientTransaction(CurrentVersion,
		Instruction{
			InstanceID: NewInstanceID(d2.GetBaseID()),
			Spawn: &Spawn{
				ContractID: "dummy",
				Args:       Arguments{{Name: "data", Value: []byte("nothing in particular")}},
			},
			SignerCounter: []uint64{1},
		},
	)

	err = ctx.FillSignersAndSignWith(signer)
	require.NoError(t, err)

	resp, err := s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: 10,
	})
	transactionOK(t, resp, err)
}

func TestService_WrongSigner(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	in1 := createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", []byte("whatever"))
	in1.SignerCounter = []uint64{1}

	signer := darc.NewSignerEd25519(nil, nil)
	tx, err := combineInstrsAndSign(signer, in1)
	require.NoError(t, err)

	resp, err := s.services[0].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 2,
	})
	// Expect it to not be accepted, because only s.signer is in the Darc
	require.NoError(t, err)
	require.Contains(t, resp.Error, "instruction verification failed: evaluating darc: expression evaluated to false")
}

// Test that inter-instruction dependencies are correctly handled.
func TestService_Depending(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	// Create a client tx with two instructions in it where the second one
	// depends on the first one having executed.

	// First instruction: spawn a dummy value.
	in1 := createSpawnInstr(s.darc.GetBaseID(), dummyContract, "data", []byte("something to delete"))
	in1.SignerIdentities = []darc.Identity{s.signer.Identity()}
	in1.SignerCounter = []uint64{1}

	// Second instruction: delete the value we just spawned.
	in2 := Instruction{
		InstanceID: NewInstanceID(in1.Hash()),
		Delete: &Delete{
			ContractID: dummyContract,
		},
	}
	in2.SignerIdentities = []darc.Identity{s.signer.Identity()}
	in2.SignerCounter = []uint64{2}

	tx, err := combineInstrsAndSign(s.signer, in1, in2)
	require.NoError(t, err)

	resp, err := s.services[0].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 2,
	})
	transactionOK(t, resp, err)

	cdb, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	_, _, _, _, err = cdb.GetValues(in1.Hash())
	require.Error(t, err)
	require.True(t, xerrors.Is(err, errKeyNotSet))

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
	err := skipchain.RegisterVerification(c, Verify, func(newID []byte, newSB *skipchain.SkipBlock) bool {
		// Make this block arrive late compared to it's timestamp. The
		// window will be 1000ms, so sleep 1200 more, just to be sure.
		time.Sleep(2200 * time.Millisecond)
		return ser.verifySkipBlock(newID, newSB)
	})
	require.NoError(t, err)

	tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.NoError(t, err)
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
	err := skipchain.RegisterVerification(c, Verify, func(newID []byte, newSB *skipchain.SkipBlock) bool {
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
	require.NoError(t, err)

	tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.NoError(t, err)
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
	// use a bigger block interval to allow txs to be included
	// in the same block
	s := newSer(t, 2, 2*time.Second)
	defer s.local.CloseAll()

	// Get counter
	counterResponse, err := s.service().GetSignerCounters(&GetSignerCounters{
		SignerIDs:   []string{s.signer.Identity().String()},
		SkipchainID: s.genesis.SkipChainID(),
	})
	require.NoError(t, err)
	counter := uint64(counterResponse.Counters[0])

	// Create a transaction without waiting, we do not use sendTransactionWithCounter
	// because it might slow us down since it gets a proof which causes the
	// transactions to end up in two blocks.
	log.Lvl1("Create transaction and don't wait")
	counter++
	{
		tx, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, s.value, s.signer, counter)
		require.NoError(t, err)
		ser := s.services[client]
		resp, err := ser.AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   s.genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: 0,
		})
		transactionOK(t, resp, err)
	}

	log.Lvl1("Create correct transaction and wait")
	counter++
	pr, k, resp, err, err2 := sendTransactionWithCounter(t, s, client, dummyContract, 10, counter)
	transactionOK(t, resp, err)
	require.NoError(t, err2)
	require.True(t, pr.InclusionProof.Match(k))

	// We expect to see both transactions in the block in pr.
	txr, err := txResultsFromBlock(&pr.Latest)
	require.NoError(t, err)
	require.Equal(t, 2, len(txr))

	log.Lvl1("Create wrong transaction and wait")
	counter++
	pr, _, resp, err, err2 = sendTransactionWithCounter(t, s, client, invalidContract, 10, counter)
	require.NoError(t, err)
	require.Contains(t, resp.Error, "this invalid contract always returns an error")
	require.NoError(t, err2)

	// We expect to see only the refused transaction in the block in pr.
	require.True(t, len(pr.Latest.Payload) > 0)
	txr, err = txResultsFromBlock(&pr.Latest)
	require.NoError(t, err)
	require.Equal(t, len(txr), 1)
	require.False(t, txr[0].Accepted)

	log.Lvl1("Create wrong transaction, no wait")
	sendTransactionWithCounter(t, s, client, invalidContract, 0, counter)
	log.Lvl1("Create second correct transaction and wait")
	pr, k, resp, err, err2 = sendTransactionWithCounter(t, s, client, dummyContract, 10, counter)
	transactionOK(t, resp, err)
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

// Sends too many transactions to the ledger and waits for all blocks to be
// done.
func TestService_FloodLedger(t *testing.T) {
	s := newSer(t, 2, testInterval)
	defer s.local.CloseAll()

	// ask to the root service because of propagation delay
	before, err := s.service().db().GetLatestByID(s.genesis.Hash)
	require.NoError(t, err)

	log.Lvl1("Create 10 transactions and don't wait")
	n := 10
	for i := 0; i < n; i++ {
		_, _, resp, err, err2 := sendTransactionWithCounter(t, s, 0, slowContract, 0, uint64(i)+2)
		transactionOK(t, resp, err)
		require.NoError(t, err2)
	}
	// Send a last transaction and wait for it to be included
	_, _, resp, err, err2 := sendTransactionWithCounter(t, s, 0, dummyContract, 10, uint64(n)+2)
	transactionOK(t, resp, err)
	require.NoError(t, err2)

	// Suppose we need at least 2 blocks (slowContract waits 1/5 interval
	// for each execution)
	latest, err := s.service().db().GetLatestByID(s.genesis.Hash)
	require.NoError(t, err)
	if latest.Index-before.Index < 2 {
		t.Fatalf("didn't get at least 2 blocks: index before %d, index after %v", before.Index, latest.Index)
	}
}

func TestService_BigTx(t *testing.T) {
	// Use longer block interval for this test, as sending around these big
	// blocks gets to be too close to the edge with the normal short
	// testing interval, and starts generating
	// errors-that-might-not-be-errors.
	s := newSer(t, 1, 2*time.Second)
	defer s.local.CloseAll()

	smallVal := s.value

	// Try to send a value so big it will be refused.
	s.value = make([]byte, defaultMaxBlockSize+1)
	_, _, _, e1, e2 := sendTransaction(t, s, 0, dummyContract, 0)
	require.Error(t, e1)
	require.Contains(t, "transaction too large", e1.Error())
	require.NoError(t, e2)

	// Now send values that are 3/4 as big as one block.
	s.value = make([]byte, defaultMaxBlockSize/4*3)

	log.Lvl1("Create 2 giant transactions and 1 little one, wait for the 3rd one")
	_, _, resp, e1, e2 := sendTransactionWithCounter(t, s, 0, dummyContract, 0, 1)
	transactionOK(t, resp, e1)
	require.NoError(t, e2)
	_, _, resp, e1, e2 = sendTransactionWithCounter(t, s, 0, dummyContract, 0, 2)
	transactionOK(t, resp, e1)
	require.NoError(t, e2)

	// Back to little values again for the last tx.
	s.value = smallVal
	p, k, resp, e1, e2 := sendTransactionWithCounter(t, s, 0, dummyContract, 10, 3)
	transactionOK(t, resp, e1)
	require.NoError(t, e2)
	require.True(t, p.InclusionProof.Match(k))

	// expect that the 2 last txns went into block #2.
	require.Equal(t, 2, p.Latest.Index)

	txr, err := txResultsFromBlock(&p.Latest)
	require.NoError(t, err)
	require.Equal(t, 2, len(txr))
}

func sendTransaction(t *testing.T, s *ser, client int, kind string, wait int) (Proof, []byte, *AddTxResponse, error, error) {
	counterResponse, err := s.service().GetSignerCounters(&GetSignerCounters{
		SignerIDs:   []string{s.signer.Identity().String()},
		SkipchainID: s.genesis.SkipChainID(),
	})
	require.NoError(t, err)
	return sendTransactionWithCounter(t, s, client, kind, wait, counterResponse.Counters[0]+1)
}

func sendTransactionWithCounter(t *testing.T, s *ser, client int, kind string, wait int, counter uint64) (Proof, []byte, *AddTxResponse, error, error) {
	tx, err := createOneClientTxWithCounter(s.darc.GetBaseID(), kind, s.value, s.signer, counter)
	require.NoError(t, err)
	ser := s.services[client]
	var resp *AddTxResponse
	resp, err = ser.AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: wait,
	})

	for isProcessing := true; isProcessing && wait != 0; {
		isProcessing = ser.skService().ChainIsProcessing(s.genesis.SkipChainID())
		time.Sleep(s.interval)
	}

	rep, err2 := ser.GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.SkipChainID(),
		Key:     tx.Instructions[0].Hash(),
	})

	var proof Proof
	if rep != nil {
		proof = rep.Proof
	}

	return proof, tx.Instructions[0].Hash(), resp, err, err2
}

func (s *ser) sendInstructions(t *testing.T, wait int,
	instr ...Instruction) (resp *AddTxResponse, ctx ClientTransaction) {
	var err error
	ctx, err = combineInstrsAndSign(s.signer, instr...)
	require.NoError(t, err)
	resp, err = s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: wait,
	})
	require.NoError(t, err)
	return
}

func TestService_InvalidVerification(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	for _, s := range s.services {
		s.testRegisterContract(panicContract, adaptor(panicContractFunc))
	}

	// tx0 uses the panicing contract, so it should _not_ be stored.
	value1 := []byte("a")
	tx0, err := createOneClientTx(s.darc.GetBaseID(), "panic", value1, s.signer)
	require.NoError(t, err)
	akvresp, err := s.service().AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.genesis.SkipChainID(),
		Transaction: tx0,
	})
	transactionOK(t, akvresp, err)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// tx1 uses the invalid contract, so it should _not_ be stored.
	tx1, err := createOneClientTx(s.darc.GetBaseID(), invalidContract, value1, s.signer)
	require.NoError(t, err)
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.genesis.SkipChainID(),
		Transaction: tx1,
	})
	transactionOK(t, akvresp, err)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// tx2 uses the dummy kind, its value should be stored.
	value2 := []byte("b")
	tx2, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, value2, s.signer)
	require.NoError(t, err)
	akvresp, err = s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx2,
		InclusionWait: 10,
	})
	transactionOK(t, akvresp, err)
	require.Equal(t, CurrentVersion, akvresp.Version)

	// Check that tx1 is _not_ stored.
	pr, err := s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.SkipChainID(),
		Key:     tx1.Instructions[0].Hash(),
	})
	require.NoError(t, err)
	match := pr.Proof.InclusionProof.Match(tx1.Instructions[0].Hash())
	require.False(t, match)

	// Check that tx2 is stored.
	pr, err = s.service().GetProof(&GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.SkipChainID(),
		Key:     tx2.Instructions[0].Hash(),
	})
	require.NoError(t, err)
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
	require.NoError(t, err)
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
				return nil, nil, xerrors.New("can only spawn add contracts")
			}
			binary.PutVarint(zeroBuf, 0)
			return []StateChange{
				{
					StateAction: Create,
					InstanceID:  inst.DeriveID("add").Slice(),
					ContractID:  cid,
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
				{
					StateAction: Update,
					InstanceID:  inst.InstanceID.Slice(),
					ContractID:  cid,
					Value:       vBuf,
				},
			}, nil, nil
		}
		return nil, nil, xerrors.New("need spawn or invoke")
	}
	s.service().testRegisterContract("add", adaptorNoVerify(f))

	cdb, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, cdb)

	// Manually create the add contract
	inst := genID()
	err = cdb.StoreAll([]StateChange{{
		StateAction: Create,
		InstanceID:  inst.Slice(),
		ContractID:  "add",
		Value:       make([]byte, 8),
	}}, 0, CurrentVersion)
	require.NoError(t, err)

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

	timestamp := time.Now().UnixNano()
	_, txOut, scs, _ := s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ct1, ct2), noTimeout, CurrentVersion, timestamp)
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
			{
				StateAction: Create,
				InstanceID:  inst.DeriveID("").Slice(),
				ContractID:  cid,
				Value:       zeroBuf,
			},
			{
				StateAction: Update,
				InstanceID:  inst.DeriveID("").Slice(),
				ContractID:  cid,
				Value:       zeroBuf,
			},
			{
				StateAction: sa,
				InstanceID:  iid2.Slice(),
				ContractID:  cid,
				Value:       zeroBuf,
			},
		}, nil, nil
	}
	require.NoError(t, s.service().testRegisterContract(cid, adaptorNoVerify(f)))
	cdb, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, cdb)

	// Create iid so we can send instructions to it
	err = cdb.StoreAll([]StateChange{{
		StateAction: Create,
		InstanceID:  iid.Slice(),
		ContractID:  cid,
		Value:       make([]byte, 8),
	}}, 0, CurrentVersion)
	require.NoError(t, err)

	timestamp := time.Now().UnixNano()

	log.Lvl1("Failing updating and removing non-existing instances")
	mkroot1, txOut, scs, _ := s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Invoke:     &Invoke{},
	}}}), noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 0, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, false, txOut[0].Accepted)
	mkroot2, txOut, scs, _ := s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Delete:     &Delete{},
	}}}), noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 0, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, false, txOut[0].Accepted)
	require.True(t, bytes.Equal(mkroot1, mkroot2))

	log.Lvl1("Create new instance, but fail to create it twice")
	txs := NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Spawn:      &Spawn{ContractID: cid},
	}}})
	mkroot1, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), txs, noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 3, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, true, txOut[0].Accepted)
	require.Nil(t, cdb.StoreAll(scs, 0, CurrentVersion))
	// Clear cache so that the transactions get re-evaluated
	delete(s.service().stateChangeCache.cache, string(s.genesis.SkipChainID()))
	mkroot2, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), txs, noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 0, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, false, txOut[0].Accepted)
	require.True(t, bytes.Equal(mkroot1, mkroot2))

	log.Lvl1("Accept updating and removing existing instance")
	_, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Invoke:     &Invoke{},
	}}}), noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 3, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, true, txOut[0].Accepted)
	_, txOut, scs, _ = s.service().createStateChanges(cdb.MakeStagingStateTrie(), s.genesis.SkipChainID(), NewTxResults(ClientTransaction{Instructions: Instructions{{
		InstanceID: iid,
		Delete:     &Delete{},
	}}}), noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 3, len(scs))
	require.Equal(t, 1, len(txOut))
	require.Equal(t, true, txOut[0].Accepted)
}

func TestService_DarcEvolutionFail(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	d2 := s.darc.Copy()
	require.Nil(t, d2.EvolveFrom(s.darc))

	// first try to evolve with the wrong contract ID
	{
		counterResponse, err := s.service().GetSignerCounters(&GetSignerCounters{
			SignerIDs:   []string{s.signer.Identity().String()},
			SkipchainID: s.genesis.SkipChainID(),
		})
		require.NoError(t, err)

		d2Buf, err := d2.ToProto()
		require.NoError(t, err)
		invoke := Invoke{
			// Because field ContractID is missing, this Invoke should fail.
			Command: cmdDarcEvolve,
			Args: []Argument{
				{
					Name:  "darc",
					Value: d2Buf,
				},
			},
		}
		instr := Instruction{
			InstanceID:    NewInstanceID(d2.GetBaseID()),
			Invoke:        &invoke,
			SignerCounter: []uint64{counterResponse.Counters[0] + 1},
		}
		resp, _ := s.sendInstructions(t, 10, instr)
		require.Contains(t, resp.Error, "instruction verification failed")
	}

	// then we create a bad request, i.e., with an invalid version number
	d2.Version = 11
	pr := s.testDarcEvolution(t, *d2, true)

	// parse the darc
	require.True(t, pr.InclusionProof.Match(d2.GetBaseID()))
	_, v0, _, _, err := pr.KeyValue()
	require.NoError(t, err)
	d22, err := darc.NewFromProtobuf(v0)
	require.NoError(t, err)
	require.False(t, d22.Equal(d2))
	require.True(t, d22.Equal(s.darc))

	// finally we do it correctly
	d2.Version = 1
	s.testDarcEvolution(t, *d2, false)
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
	require.NoError(t, err)
	d22, err := darc.NewFromProtobuf(v0)
	require.NoError(t, err)
	require.True(t, d22.Equal(d2))
}

func TestService_DarcSpawn(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	id := []darc.Identity{s.signer.Identity()}
	darc2 := darc.NewDarc(darc.InitRulesWith(id, id, "invoke:"+ContractDarcID+"."+cmdDarcEvolveUnrestriction),
		[]byte("next darc"))
	darc2.Rules.AddRule("spawn:rain", darc2.Rules.GetSignExpr())
	darc2Buf, err := darc2.ToProto()
	require.NoError(t, err)
	darc2Copy, err := darc.NewFromProtobuf(darc2Buf)
	require.NoError(t, err)
	require.True(t, darc2.Equal(darc2Copy))

	ctx := NewClientTransaction(CurrentVersion,
		Instruction{
			InstanceID: NewInstanceID(s.darc.GetBaseID()),
			Spawn: &Spawn{
				ContractID: ContractDarcID,
				Args: []Argument{{
					Name:  "darc",
					Value: darc2Buf,
				}},
			},
			SignerIdentities: []darc.Identity{s.signer.Identity()},
			SignerCounter:    []uint64{1},
		},
	)
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
	darc2.Rules.AddRule("spawn:"+ContractDarcID, expression.InitOrExpr(s.darc.GetIdentityString()))
	darc2Buf, err := darc2.ToProto()
	require.NoError(t, err)
	darc2Copy, err := darc.NewFromProtobuf(darc2Buf)
	require.NoError(t, err)
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
	s.sendInstructions(t, 10, instr)
	pr := s.waitProof(t, NewInstanceID(darc2.GetBaseID()))
	require.True(t, pr.InclusionProof.Match(darc2.GetBaseID()))

	// Spawn third darc from the second one, but sign the request with
	// the signer of the first darc to test delegation
	signer3 := darc.NewSignerEd25519(nil, nil)
	id3 := []darc.Identity{signer3.Identity()}
	darc3 := darc.NewDarc(darc.InitRules(id3, id3),
		[]byte("third darc"))
	darc3Buf, err := darc3.ToProto()
	require.NoError(t, err)
	darc3Copy, err := darc.NewFromProtobuf(darc3Buf)
	require.NoError(t, err)
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
	s.sendInstructions(t, 10, instr)
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
	darc2.Rules.AddRule("spawn:"+ContractDarcID, expression.Expr(s.darc.GetIdentityString()))
	darc2Buf, err := darc2.ToProto()
	require.NoError(t, err)
	darc2Copy, err := darc.NewFromProtobuf(darc2Buf)
	require.NoError(t, err)
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
	s.sendInstructions(t, 10, instr)
	pr := s.waitProof(t, NewInstanceID(darc2.GetBaseID()))
	require.True(t, pr.InclusionProof.Match(darc2.GetBaseID()))

	ca := &CheckAuthorization{
		Version:    CurrentVersion,
		ByzCoinID:  s.genesis.SkipChainID(),
		DarcID:     s.darc.GetBaseID(),
		Identities: []darc.Identity{s.signer.Identity()},
	}
	resp, err := s.service().CheckAuthorization(ca)
	require.NoError(t, err)
	require.Contains(t, resp.Actions, darc.Action("_sign"))

	ca.Identities[0] = darc.NewIdentityEd25519(s.roster.List[0].Public)
	resp, err = s.service().CheckAuthorization(ca)
	require.NoError(t, err)
	require.Contains(t, resp.Actions, darc.Action("invoke:"+ContractConfigID+".view_change"))

	ca.Identities = append(ca.Identities, darc.NewIdentityEd25519(s.roster.List[1].Public))
	resp, err = s.service().CheckAuthorization(ca)
	require.NoError(t, err)
	require.Contains(t, resp.Actions, darc.Action("invoke:"+ContractConfigID+".view_change"))

	log.Lvl1("Check delegation of darcs")
	ca.DarcID = darc2.GetID()
	ca.Identities[0] = darc.NewSignerEd25519(nil, nil).Identity()
	resp, err = s.service().CheckAuthorization(ca)
	require.NoError(t, err)
	require.Equal(t, 0, len(resp.Actions))

	ca.DarcID = darc2.GetID()
	ca.Identities[0] = s.signer.Identity()
	resp, err = s.service().CheckAuthorization(ca)
	require.NoError(t, err)
	require.Contains(t, resp.Actions, darc.Action("spawn:"+ContractDarcID))

	ca.DarcID = darc2.GetID()
	ca.Identities[0] = darc.NewIdentityDarc(s.darc.GetID())
	resp, err = s.service().CheckAuthorization(ca)
	require.NoError(t, err)
	require.Contains(t, resp.Actions, darc.Action("spawn:"+ContractDarcID))
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
	require.NoError(t, err)
	s.sendTxAndWait(t, ctx, 10)

	intervals := []time.Duration{
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
	}
	if testing.Short() {
		intervals = intervals[0:2]
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

		// We send an extra transaction first because the new interval is only loaded after a delay
		// caused by the pipeline feature, i.e., the new interval is only used after an existing wait-interval
		// is finished and not immediately after receiving the new configuration.
		dummyCtx, _ := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(counter))
		counter++
		s.sendTxAndWait(t, dummyCtx, 10)

		start := time.Now()

		dummyCtx, _ = createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(counter))
		counter++
		s.sendTxAndWait(t, dummyCtx, 10)

		dur := time.Since(start)
		require.InDelta(t, dur, interval, float64(1*time.Second))
	}
}

func TestService_SetConfigRosterKeepLeader(t *testing.T) {
	n := 6
	if testing.Short() {
		n = 2
	}

	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	log.Lvl1("Creating blocks to check rotation of the roster while keeping leader")
	rosterR := s.roster
	for i := 0; i < n; i++ {
		rosterR = onet.NewRoster([]*network.ServerIdentity{
			rosterR.List[0], rosterR.List[2], rosterR.List[3], rosterR.List[1]})
		log.Lvl2("Creating block", i)
		ctx, _ := createConfigTxWithCounter(t, testInterval, *rosterR, defaultMaxBlockSize, s, 1+i)
		s.sendTxAndWait(t, ctx, 10)
		log.Lvl2("Verifying the correct roster is in place")
		latest, err := s.service().db().GetLatestByID(s.genesis.Hash)
		require.NoError(t, err)
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
		require.NoError(t, err)
		require.True(t, latest.Roster.ID.Equal(rosterR.ID), "roster has not been updated")
	}
}

func TestService_SetConfigRosterNewNodes(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()
	nbrNewNodes := 10
	if testing.Short() {
		nbrNewNodes = 2
	}

	servers, newRoster, _ := s.local.MakeSRS(cothority.Suite, nbrNewNodes, ByzCoinID)

	ids := []darc.Identity{s.signer.Identity()}
	testDarc := darc.NewDarc(darc.InitRules(ids, ids), []byte("testDarc"))
	testDarcBuf, err := testDarc.ToProto()
	require.NoError(t, err)
	instr := createSpawnInstr(s.darc.GetBaseID(), ContractDarcID, "darc", testDarcBuf)
	require.NoError(t, err)
	s.sendInstructions(t, 10, instr)

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
		require.NoError(t, err)
		require.True(t, latest.Roster.ID.Equal(rosterR.ID), "roster has not been updated")
		// Get latest genesis darc and verify the 'view_change' rule is updated
		st, err := s.service().GetReadOnlyStateTrie(s.genesis.Hash)
		require.NoError(t, err)
		val, _, _, _, err := st.GetValues(s.darc.GetBaseID())
		require.NoError(t, err)
		d, err := darc.NewFromProtobuf(val)
		require.NoError(t, err)
		vcIDs := strings.Split(string(d.Rules.Get("invoke:"+ContractConfigID+".view_change")), " | ")
		require.Equal(t, len(rosterR.List), len(vcIDs))
	}

	// Make sure the latest node is correctly activated and that the
	// new conodes are done with catching up
	for _, ser := range servers {
		ctx, _ := createConfigTxWithCounter(t, testInterval, *rosterR,
			defaultMaxBlockSize, s, counter)
		counter++
		for i := 0; i < 2; i++ {
			resp, err := ser.Service(ServiceName).(*Service).AddTransaction(&AddTxRequest{
				Version:       CurrentVersion,
				SkipchainID:   s.genesis.SkipChainID(),
				Transaction:   ctx,
				InclusionWait: 10,
			})
			if err == nil && resp.Error == "" {
				break
			} else if i == 2 {
				transactionOK(t, resp, err)
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
				require.NoError(t, err)
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
	resp, err := s.services[0].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: 10,
	})
	require.NoError(t, err)
	require.Contains(t, resp.Error, "new leader must be in previous roster")

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
		resp, err := cl.AddTransactionAndWait(ctx, 10)
		transactionOK(t, resp, err)

		log.Lvl1("Removing", goodRoster.List[0])
		goodRoster = onet.NewRoster(goodRoster.List[1:])
		ctx, _ = createConfigTxWithCounter(t, testInterval, *goodRoster, defaultMaxBlockSize, s, counter)
		counter++
		resp, err = cl.AddTransactionAndWait(ctx, 10)
		transactionOK(t, resp, err)
	}
}

func addDummyTxs(t *testing.T, s *ser, nbr int, perCTx int, count int) int {
	return addDummyTxsTo(t, s, nbr, perCTx, count, 0)
}
func addDummyTxsTo(t *testing.T, s *ser, nbr int, perCTx int, count int, idx int) int {
	ids := []darc.Identity{s.signer.Identity()}
	for i := 0; i < nbr; i++ {
		var instrs Instructions
		for j := 0; j < perCTx; j++ {
			desc := random.Bits(256, true, random.New())
			dummyDarc := darc.NewDarc(darc.InitRules(ids, ids), desc)
			dummyDarcBuf, err := dummyDarc.ToProto()
			require.NoError(t, err)
			instr := createSpawnInstr(s.darc.GetBaseID(), ContractDarcID,
				"darc", dummyDarcBuf)
			instr.SignerCounter[0] = uint64(count)
			count++
			instrs = append(instrs, instr)
		}
		ctx, err := combineInstrsAndSign(s.signer, instrs...)
		require.NoError(t, err)

		s.sendTxToAndWait(t, ctx, idx, 10)
		s.sendInstructions(t, 10, instrs...)
		s.local.WaitDone(time.Second)
	}
	return count
}

func TestService_SetConfigRosterDownload(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	cfdb := catchupFetchDBEntries
	defer func() {
		catchupFetchDBEntries = cfdb
	}()
	catchupFetchDBEntries = 10

	ids := []darc.Identity{s.signer.Identity()}
	testDarc := darc.NewDarc(darc.InitRules(ids, ids), []byte("testDarc"))
	testDarcBuf, err := testDarc.ToProto()
	require.NoError(t, err)
	instr := createSpawnInstr(s.darc.GetBaseID(), ContractDarcID, "darc", testDarcBuf)
	require.NoError(t, err)
	s.sendInstructions(t, 10, instr)
	// Add other transaction so we're on a new border between forward links
	ct := addDummyTxs(t, s, 4, 1, 2)

	cda := catchupDownloadAll
	defer func() {
		catchupDownloadAll = cda
	}()
	catchupDownloadAll = 1
	_, newRoster, _ := s.local.MakeSRS(cothority.Suite, 1, ByzCoinID)

	newRoster = onet.NewRoster(append(s.roster.List, newRoster.List...))
	ctx, _ := createConfigTxWithCounter(t, testInterval, *newRoster,
		defaultMaxBlockSize, s, ct)
	ct++
	s.sendTxAndWait(t, ctx, 10)

	// Create a new block
	log.Lvl1("Creating two dummy blocks for the new node to catch up")
	addDummyTxs(t, s, 2, 1, ct)

	log.Lvl1("And getting proof from new node that the testDarc exists")
	leanClient := onet.NewClient(cothority.Suite, ServiceName)
	reply := &GetProofResponse{}
	err = leanClient.SendProtobuf(newRoster.List[len(newRoster.List)-1], &GetProof{
		Version: CurrentVersion,
		ID:      s.genesis.Hash,
		Key:     testDarc.GetBaseID(),
	}, reply)
	require.NoError(t, err)
	require.True(t, reply.Proof.InclusionProof.Match(testDarc.GetBaseID()))
}

func TestService_DownloadState(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	cfdb := catchupFetchDBEntries
	defer func() {
		catchupFetchDBEntries = cfdb
	}()
	catchupFetchDBEntries = 10

	log.Lvl1("Adding dummy transactions")
	ct := addDummyTxs(t, s, 3, 3, 1)
	addDummyTxs(t, s, 1, 20, ct)

	config, err := s.service().LoadConfig(s.genesis.SkipChainID())
	require.NoError(t, err)
	stateTrie, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	merkleRoot := stateTrie.GetRoot()

	// Wrong parameters
	log.Lvl1("Testing wrong parameters")
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: skipchain.SkipBlockID{},
	})
	require.Error(t, err)
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: skipchain.SkipBlockID{},
		Nonce:     0,
		Length:    1,
	})
	require.Error(t, err)
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
	})
	require.Error(t, err)
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     1,
	})
	require.Error(t, err)
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
	})
	require.Error(t, err)

	// Start one download and check it is aborted
	// if we start a second download.
	log.Lvl1("Check aborting of download and resuming")
	resp, err := s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
		Length:    1,
	})
	require.NoError(t, err)
	nonce1 := resp.Nonce
	// Continue 1st download
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     nonce1,
		Length:    1,
	})
	require.NoError(t, err)
	// Start 2nd download
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
		Length:    1,
	})
	require.NoError(t, err)
	nonce2 := resp.Nonce
	require.NotEqual(t, nonce1, nonce2)
	// Now 1st download should fail
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     nonce1,
		Length:    1,
	})
	require.Error(t, err)
	// And 2nd download should still continue
	_, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     nonce2,
		Length:    1,
	})
	require.NoError(t, err)

	// Start downloading
	log.Lvl1("Partial download")
	resp, err = s.service().DownloadState(&DownloadState{
		ByzCoinID: s.genesis.SkipChainID(),
		Nonce:     0,
		Length:    10,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 10, len(resp.KeyValues))

	// Start a new download and go till the end
	log.Lvl1("Full download")
	length := 0
	var nonce uint64
	for {
		resp, err = s.service().DownloadState(&DownloadState{
			ByzCoinID: s.genesis.SkipChainID(),
			Nonce:     nonce,
			Length:    10,
		})
		require.NoError(t, err)
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
	configDown, err := s.service().LoadConfig(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.Equal(t, config, configDown)
	stateTrieDown, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	merkleRootDown := stateTrieDown.GetRoot()
	require.Equal(t, merkleRoot, merkleRootDown)

	time.Sleep(time.Second)
	// Try to re-create the trie on a new service -
	// do it twice
	for i := 0; i < 2; i++ {
		log.Lvl1("Full download on new node, try-#", i+1)
		servers, _, _ := s.local.MakeSRS(cothority.Suite, 1, ByzCoinID)
		services := s.local.GetServices(servers, ByzCoinID)
		service := services[0].(*Service)
		err := service.downloadDB(s.genesis)
		require.NoError(t, err)
		st, err := service.getStateTrie(s.genesis.Hash)
		require.NoError(t, err)
		val, _, _, _, err := st.GetValues(make([]byte, 32))
		require.NoError(t, err)
		require.True(t, len(val) > 0)
		configCopy := ChainConfig{}
		err = protobuf.DecodeWithConstructors(val, &configCopy, network.DefaultConstructors(cothority.Suite))
		require.NoError(t, err)
		require.Equal(t, config, &configCopy)
		stateTrieDown, err := service.getStateTrie(s.genesis.SkipChainID())
		require.NoError(t, err)
		merkleRootDown := stateTrieDown.GetRoot()
		require.Equal(t, merkleRoot, merkleRootDown)
	}
}

// Download the state in a running Byzcoin, with a node sudeenly being caught by Amnesia.
// This is different from the above tests, as a node needs to be able to catch up
// while a full running byzcoin is in place.
//
// Two things are not tested here:
//   1. what if a leader fails and wants to catch up
//   2. if the catchupFetchDBEntries = 1, it fails
func TestService_DownloadStateRunning(t *testing.T) {

	// Disabled because it is flaky. See issue.
	t.Skip("https://github.com/dedis/cothority/issues/2129")

	cda := catchupDownloadAll
	defer func() {
		catchupDownloadAll = cda
	}()
	catchupDownloadAll = 3
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	log.Lvl1("Adding dummy transactions")
	addDummyTxs(t, s, 3, 1, 1)

	counter := 4
	for i := range s.services {
		if i == 0 {
			log.Lvl1("Not deleting leader")
			continue
		}
		log.Lvl1("Deleting node", i, "and adding new transaction")
		s.deleteDBs(t, i)

		addDummyTxsTo(t, s, 1, 1, counter, (i+1)%len(s.services))
		counter++
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
		service.closed = false
		require.NoError(t, service.startChain(scID))
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
	require.NoError(t, s.service().testRegisterContract(contractID, adaptor(contract)))

	scID := s.genesis.SkipChainID()
	st, err := s.service().getStateTrie(scID)
	require.NoError(t, err)
	sst := st.MakeStagingStateTrie()
	tx1, err := createOneClientTxWithCounter(s.darc.GetBaseID(), contractID, []byte{}, s.signer, 1)
	require.NoError(t, err)

	// Add a second tx that is invalid because it is for an unknown contract.
	log.Lvl1("Calling invalid invoke on contract")
	tx2, err := createOneClientTxWithCounter(s.darc.GetBaseID(), contractID+"x", []byte{}, s.signer, 2)
	require.NoError(t, err)

	timestamp := time.Now().UnixNano()

	txs := NewTxResults(tx1, tx2)
	require.NoError(t, err)
	root, txOut, states, _ := s.service().createStateChanges(sst, scID, txs, noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 2, len(txOut))
	require.Equal(t, 1, ctr)
	// we expect one state change to increment the signature counter
	require.Equal(t, 1, len(states))
	require.Equal(t, "", states[0].ContractID)
	require.Equal(t, []byte{}, []byte(states[0].DarcID))

	// If we call createStateChanges on the new txOut (as it will happen in production
	// when the tx set is reduced by the selection step, and then txOut are sent to
	// createStateChanges when making the block), then it should load it from the
	// cache, which means that ctr is still one (we do not call the
	// contract twice).
	root1, txOut1, states1, _ := s.service().createStateChanges(sst, scID, txOut, noTimeout, CurrentVersion, timestamp)
	require.Equal(t, 1, ctr)
	require.Equal(t, root, root1)
	require.Equal(t, txOut, txOut1)
	require.Equal(t, states, states1)

	// If we remove the cache, then we expect the contract to be called
	// again, i.e., ctr == 2.
	s.service().stateChangeCache = newStateChangeCache()
	require.NoError(t, err)
	root2, txOut2, states2, _ := s.service().createStateChanges(sst, scID, txs, noTimeout, CurrentVersion, timestamp)
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
	require.NoError(t, err)
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
		if _, _, _, _, err := cdb.GetValues(iid.Slice()); xerrors.Is(err, errKeyNotSet) {
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
	for _, s := range s.services {
		s.testRegisterContract(contractID, adaptor(contract))
	}

	for i := 0; i < n; i++ {
		tx, err := createClientTxWithTwoInstrWithCounter(s.darc.GetBaseID(), contractID, []byte{}, s.signer, uint64(i*2+1))
		require.NoError(t, err)

		// Queue all transactions, except for the last one
		wait := 0
		if i == n-1 {
			wait = 10
		}
		resp, err := s.service().AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   s.genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: wait,
		})
		transactionOK(t, resp, err)
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
			require.NoError(t, err)
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
		require.NoError(t, err)
		require.Equal(t, 2*n+1, len(res.StateChanges))

		log.Lvlf1("Getting versions of iid %x and signer %x", iid[:], signerIID[:])
		for i := 0; i < n*2; i++ {
			log.Lvlf1("Getting version %d", i)
			sc, err := service.GetInstanceVersion(&GetInstanceVersion{
				SkipChainID: scID,
				InstanceID:  iid,
				Version:     uint64(i),
			})
			require.NoError(t, err)
			require.Equal(t, uint64(i), sc.StateChange.Version)
			require.Equal(t, uint64(i), res.StateChanges[i].StateChange.Version)

			res, err := service.CheckStateChangeValidity(&CheckStateChangeValidity{
				SkipChainID: scID,
				InstanceID:  iid,
				Version:     uint64(i),
			})
			require.NoError(t, err)

			sb, err := service.skService().GetSingleBlock(&skipchain.GetSingleBlock{ID: res.BlockID})
			require.NoError(t, err)
			var header DataHeader
			err = protobuf.Decode(sb.Data, &header)
			require.NoError(t, err)
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

// Tests that the state change storage will be caught up by a new conode
func TestService_StateChangeStorageCatchUp(t *testing.T) {
	cda := catchupDownloadAll
	defer func() {
		catchupDownloadAll = cda
	}()
	// we don't want a db download
	catchupDownloadAll = 100

	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	n := 2
	for i := 0; i < n; i++ {
		tx, err := createClientTxWithTwoInstrWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(i*2+1))
		require.NoError(t, err)

		resp, err := s.service().AddTransaction(&AddTxRequest{
			Version:       CurrentVersion,
			SkipchainID:   s.genesis.SkipChainID(),
			Transaction:   tx,
			InclusionWait: 10,
		})
		transactionOK(t, resp, err)
	}

	newServer, newRoster, newService := s.local.MakeSRS(cothority.Suite, 1, ByzCoinID)
	registerDummy(t, newServer)

	newRoster = onet.NewRoster(append(s.roster.List, newRoster.List...))
	ctx, _ := createConfigTxWithCounter(t, testInterval, *newRoster, defaultMaxBlockSize, s, 5)
	log.Lvl1("Updating config to include new roster")
	s.sendTxAndWait(t, ctx, 10)

	for i := 0; i < 10; i++ {
		log.Lvl1("Sleeping for 1 second...")
		time.Sleep(time.Second)
		scs, _ := newService.(*Service).stateChangeStorage.getByBlock(s.genesis.Hash, 2)

		if len(scs) > 0 {
			require.Equal(t, Create, scs[0].StateChange.StateAction)
			require.Equal(t, Update, scs[1].StateChange.StateAction)
			require.Equal(t, uint64(3), scs[1].StateChange.Version)
			return
		}
	}

	require.True(t, false, "the new conode has never caught up in the last 10s")
}

// Tests that a conode can't be overflowed by catching requests
func TestService_TestCatchUpHistory(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	require.Equal(t, 0, len(s.service().catchingUpHistory))

	sc := s.service().Service(skipchain.ServiceName).(*skipchain.Service)
	sc.Storage.FollowIDs = []skipchain.SkipBlockID{s.genesis.Hash}

	// unknown, unfriendly skipchain, we shouldn't try to catch up
	err := s.service().catchupFromID(s.roster, skipchain.SkipBlockID{}, skipchain.SkipBlockID{})
	require.Equal(t, 0, len(s.service().catchingUpHistory))
	require.NoError(t, err)

	// catch up
	err = s.service().catchupFromID(s.roster, s.genesis.Hash, s.genesis.Hash)
	require.Equal(t, 1, len(s.service().catchingUpHistory))
	require.NoError(t, err)

	ts := s.service().catchingUpHistory[string(s.genesis.Hash)]

	// ... but not twice
	err = s.service().catchupFromID(s.roster, s.genesis.Hash, s.genesis.Hash)
	require.True(t, s.service().catchingUpHistory[string(s.genesis.Hash)].Equal(ts))
	require.Error(t, err)
}

func TestService_Repair(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	var intermediateStateTrie *stateTrie
	var finalRoot []byte
	n := 5
	for i := 0; i < n; i++ {
		ctx, err := createOneClientTxWithCounter(s.darc.GetBaseID(), dummyContract, []byte{}, s.signer, uint64(i+1))
		require.NoError(t, err)
		s.sendTxAndWait(t, ctx, 10)

		// take a copy of the state trie at the middle
		if i == 2 {
			tmpTrie, err := s.service().getStateTrie(s.genesis.SkipChainID())
			require.NoError(t, err)
			nonce, err := tmpTrie.GetNonce()
			require.NoError(t, err)
			intermediateTrie, err := trie.NewTrie(trie.NewMemDB(), nonce)
			require.NoError(t, err)

			err = intermediateTrie.DB().Update(func(b trie.Bucket) error {
				return tmpTrie.CopyTo(b)
			})
			require.NoError(t, err)

			intermediateStateTrie = &stateTrie{*intermediateTrie, trieCache{}}
		} else if i == n-1 {
			tmpTrie, err := s.service().getStateTrie(s.genesis.SkipChainID())
			require.NoError(t, err)
			finalRoot = tmpTrie.GetRoot()
		}
	}

	// Introduce an artificial corruption and then try to repair it.
	genesisHex := fmt.Sprintf("%x", s.genesis.SkipChainID())
	s.service().stateTriesLock.Lock()
	s.service().stateTries[genesisHex] = intermediateStateTrie
	s.service().stateTriesLock.Unlock()

	err := s.service().fixInconsistencyIfAny(s.genesis.SkipChainID(), intermediateStateTrie)
	require.NoError(t, err)

	s.service().stateTriesLock.Lock()
	newRoot := s.service().stateTries[genesisHex].GetRoot()
	s.service().stateTriesLock.Unlock()
	require.Equal(t, finalRoot, newRoot)
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
		BlockInterval:   interval,
		Roster:          roster,
		MaxBlockSize:    size,
		DarcContractIDs: []string{ContractDarcID},
	}
	configBuf, err := protobuf.Encode(&config)
	require.NoError(t, err)

	instr := Instruction{
		InstanceID: NewInstanceID(nil),
		Invoke: &Invoke{
			ContractID: ContractConfigID,
			Command:    "update_config",
			Args: []Argument{{
				Name:  "config",
				Value: configBuf,
			}},
		},
		SignerIdentities: []darc.Identity{s.signer.Identity()},
		SignerCounter:    []uint64{uint64(counter)},
		version:          CurrentVersion,
	}
	ctx, err := combineInstrsAndSign(s.signer, instr)

	require.NoError(t, err)
	return ctx, config
}

func darcToTx(t *testing.T, d2 darc.Darc, signer darc.Signer, ctr uint64) ClientTransaction {
	d2Buf, err := d2.ToProto()
	require.NoError(t, err)
	invoke := Invoke{
		ContractID: ContractDarcID,
		Command:    cmdDarcEvolve,
		Args: []Argument{
			{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := Instruction{
		InstanceID:    NewInstanceID(d2.GetBaseID()),
		Invoke:        &invoke,
		SignerCounter: []uint64{ctr},
		version:       CurrentVersion,
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
	resp, err := s.services[idx].AddTransaction(&AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: s.genesis.SkipChainID(),
		Transaction: ctx,
	})
	transactionOK(t, resp, err)
}

func (s *ser) sendTxAndWait(t *testing.T, ctx ClientTransaction, wait int) {
	s.sendTxToAndWait(t, ctx, 0, wait)
}

func (s *ser) sendTxToAndWait(t *testing.T, ctx ClientTransaction, idx int, wait int) {
	resp, err := s.services[idx].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   ctx,
		InclusionWait: wait,
	})
	transactionOK(t, resp, err)
}

// caller gives us a darc, and we try to make an evolution request.
func (s *ser) testDarcEvolution(t *testing.T, d2 darc.Darc, fail bool) (pr *Proof) {
	counterResponse, err := s.service().GetSignerCounters(&GetSignerCounters{
		SignerIDs:   []string{s.signer.Identity().String()},
		SkipchainID: s.genesis.SkipChainID(),
	})
	require.NoError(t, err)

	ctx := darcToTx(t, d2, s.signer, counterResponse.Counters[0]+1)
	s.sendTx(t, ctx)
	for i := 0; i < 10; i++ {
		resp, err := s.service().GetProof(&GetProof{
			Version: CurrentVersion,
			Key:     d2.GetBaseID(),
			ID:      s.genesis.SkipChainID(),
		})
		require.NoError(t, err)
		pr = &resp.Proof
		_, v0, _, _, err := pr.KeyValue()
		require.NoError(t, err)
		d, err := darc.NewFromProtobuf(v0)
		require.NoError(t, err)
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

func (s *ser) deleteDBs(t *testing.T, index int) {
	bc := s.services[index]
	log.Lvlf1("%s: Deleting DB of node %d", bc.ServerIdentity(), index)
	bc.TestClose()
	for scid := range bc.stateTries {
		require.NoError(t, deleteDB(bc.ServiceProcessor, []byte(scid)))
		idStr := hex.EncodeToString([]byte(scid))
		require.NoError(t, deleteDB(bc.ServiceProcessor, []byte(idStr)))
	}
	require.NoError(t, deleteDB(bc.ServiceProcessor, storageID))
	sc := bc.Service(skipchain.ServiceName).(*skipchain.Service)
	require.NoError(t, deleteDB(sc.ServiceProcessor, []byte("skipblocks")))
	require.NoError(t, deleteDB(sc.ServiceProcessor, []byte("skipchainconfig")))
	require.NoError(t, bc.TestRestart())
}

// Waits to have a coherent view in all nodes with at least the block
// 'index' held by all nodes.
func (s *ser) waitPropagation(t *testing.T, index int) {
	require.NoError(t, NewClient(s.genesis.Hash,
		*s.roster).WaitPropagation(index))
}

func deleteDB(s *onet.ServiceProcessor, key []byte) error {
	db, stBucket := s.GetAdditionalBucket(key)
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(stBucket)
	})
}

func newSer(t *testing.T, step int, interval time.Duration) *ser {
	return newSerN(t, step, interval, 4, disableViewChange)
}

func newSerN(t *testing.T, step int, interval time.Duration, n int, rw time.Duration) *ser {
	return newSerWithVersion(t, step, interval, n, rw, CurrentVersion)
}

func newSerWithVersion(t *testing.T, step int, interval time.Duration, n int, rw time.Duration, v Version) *ser {
	s := &ser{
		local:  onet.NewLocalTestT(tSuite, t),
		value:  []byte("anyvalue"),
		signer: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(n, true)
	for _, sv := range s.local.GetServices(s.hosts, ByzCoinID) {
		service := sv.(*Service)
		service.rotationWindow = rw
		service.defaultVersion = v
		s.services = append(s.services, service)
	}
	registerDummy(t, s.hosts)

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster,
		[]string{
			"spawn:" + dummyContract,
			"spawn:" + invalidContract,
			"spawn:" + panicContract,
			"spawn:" + slowContract,
			"spawn:" + versionContract,
			"spawn:" + stateChangeCacheContract,
			"delete:" + dummyContract,
		}, s.signer.Identity())
	require.NoError(t, err)
	s.darc = &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = interval
	s.interval = genesisMsg.BlockInterval

	for i := 0; i < step; i++ {
		switch i {
		case 0:
			resp, err := s.service().CreateGenesisBlock(genesisMsg)
			require.NoError(t, err)
			s.genesis = resp.Skipblock
		case 1:
			tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
			require.NoError(t, err)
			s.tx = tx
			resp, err := s.service().AddTransaction(&AddTxRequest{
				Version:       CurrentVersion,
				SkipchainID:   s.genesis.SkipChainID(),
				Transaction:   tx,
				InclusionWait: 10,
			})
			transactionOK(t, resp, err)
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
	return nil, nil, xerrors.New("this invalid contract always returns an error")
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

// Simple contract that verifies if the available version is equal to the value.
func versionContractFunc(rst ReadOnlyStateTrie, inst Instruction, c []Coin) ([]StateChange, []Coin, error) {
	_, _, _, darcID, err := rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	if rst.GetVersion() != Version(inst.Spawn.Args[0].Value[0]) {
		return nil, nil, xerrors.New("wrong byzcoin version")
	}

	sc := NewStateChange(Create, NewInstanceID(inst.Hash()), versionContract, inst.Spawn.Args[0].Value, darcID)
	return []StateChange{sc}, c, nil
}

func registerDummy(t *testing.T, servers []*onet.Server) {
	// For testing - there must be a better way to do that. But putting
	// services []skipchain.Service in the method signature doesn't work :(
	for _, s := range servers {
		service := s.Service(ServiceName).(*Service)

		err := service.testRegisterContract(dummyContract, adaptor(dummyContractFunc))
		require.NoError(t, err)
		err = service.testRegisterContract(slowContract, adaptor(slowContractFunc))
		require.NoError(t, err)
		err = service.testRegisterContract(invalidContract, adaptor(invalidContractFunc))
		require.NoError(t, err)
		err = service.testRegisterContract(versionContract, adaptor(versionContractFunc))
		require.NoError(t, err)
	}
}

func genID() (i InstanceID) {
	random.Bytes(i[:], random.New())
	return i
}

// registerContract stores the contract in a map and will
// call it whenever a contract needs to be done.
func (s *Service) testRegisterContract(contractID string, c ContractFn) error {
	s.contracts.registry[contractID] = c
	return nil
}

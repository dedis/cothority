// Package service implements the OmniLedger service.
package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/cothority/omniledger/collection"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"gopkg.in/satori/go.uuid.v1"
)

const invokeEvolve darc.Action = darc.Action("invoke:evolve")

const rotationWindow time.Duration = 5

const noTimeout time.Duration = 0

const collectTxProtocol = "CollectTxProtocol"

// OmniledgerID can be used to refer to this service
var OmniledgerID onet.ServiceID

var verifyOmniLedger = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "OmniLedger"))

func init() {
	var err error
	OmniledgerID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&omniStorage{}, &DataHeader{}, &DataBody{})
}

// GenNonce returns a random nonce.
func GenNonce() (n Nonce) {
	random.Bytes(n[:], random.New())
	return n
}

// Service is our OmniLedger-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	// collections cannot be stored, so they will be re-created whenever the
	// service reloads.
	collectionDB map[string]*collectionDB
	// holds a link for every omniledger to the latest block that is included in
	// the collection.
	// TODO: merge collectionDB and pollChan into olState structure.
	state olState

	// pollChan maintains a map of channels that can be used to stop the
	// polling go-routing.
	pollChan    map[string]chan bool
	pollChanMut sync.Mutex
	pollChanWG  sync.WaitGroup

	// NOTE: If we have a lot of skipchains, then using mutex most likely
	// will slow down our service, an improvement is to go-routines to
	// store transactions. But there is more management overhead, e.g.,
	// restarting after shutdown, answer getTxs requests and so on.
	txBuffer txBuffer

	heartbeats        heartbeats
	heartbeatsTimeout chan string
	heartbeatsClose   chan bool

	// contracts map kinds to kind specific verification functions
	contracts map[string]OmniLedgerContract
	// propagate the new transactions
	propagateTransactions messaging.PropagationFunc

	storage *omniStorage

	createSkipChainMut sync.Mutex

	darcToSc    map[string]skipchain.SkipBlockID
	darcToScMut sync.Mutex

	stateChangeCache stateChangeCache
}

// storageID reflects the data we're storing - we could store more
// than one structure.
var storageID = []byte("OmniLedger")

// defaultInterval is used if the BlockInterval field in the genesis
// transaction is not set.
var defaultInterval = 5 * time.Second

// omniStorage is used to save our data locally.
type omniStorage struct {
	// PropTimeout is used when sending the request to integrate a new block
	// to all nodes.
	PropTimeout time.Duration

	sync.Mutex
}

// CreateGenesisBlock asks the service to create a new skipchain ready to
// store key/value pairs. If it is given exactly one writer, this writer will
// be stored in the skipchain.
// For faster access, all data is also stored locally in the Service.storage
// structure.
func (s *Service) CreateGenesisBlock(req *CreateGenesisBlock) (
	*CreateGenesisBlockResponse, error) {
	// We use a big mutex here because we do not want to allow concurrent
	// creation of genesis blocks.
	// TODO an optimisation would be to lock on the skipchainID.
	s.createSkipChainMut.Lock()
	defer s.createSkipChainMut.Unlock()

	if req.Version != CurrentVersion {
		return nil, fmt.Errorf("version mismatch - got %d but need %d", req.Version, CurrentVersion)
	}
	if req.Roster.List == nil {
		return nil, errors.New("must provide a roster")
	}

	darcBuf, err := req.GenesisDarc.ToProto()
	if err != nil {
		return nil, err
	}
	if req.GenesisDarc.Verify(true) != nil ||
		req.GenesisDarc.Rules.Count() == 0 {
		return nil, errors.New("invalid genesis darc")
	}

	if req.BlockInterval == 0 {
		req.BlockInterval = defaultInterval
	}
	intervalBuf := make([]byte, 8)
	binary.PutVarint(intervalBuf, int64(req.BlockInterval))

	rosterBuf, err := protobuf.Encode(&req.Roster)
	if err != nil {
		return nil, err
	}

	spawn := &Spawn{
		ContractID: ContractConfigID,
		Args: Arguments{
			{Name: "darc", Value: darcBuf},
			{Name: "block_interval", Value: intervalBuf},
			{Name: "roster", Value: rosterBuf},
		},
	}

	// Create the genesis-transaction with a special key, it acts as a
	// reference to the actual genesis transaction.
	transaction := NewTxResults(ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: ConfigInstanceID,
			Nonce:      Nonce{},
			Index:      0,
			Length:     1,
			Spawn:      spawn,
		}},
	})

	sb, err := s.createNewBlock(nil, &req.Roster, transaction)
	if err != nil {
		return nil, err
	}

	return &CreateGenesisBlockResponse{
		Version:   CurrentVersion,
		Skipblock: sb,
	}, nil
}

// AddTransaction requests to apply a new transaction to the ledger.
func (s *Service) AddTransaction(req *AddTxRequest) (*AddTxResponse, error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	if len(req.Transaction.Instructions) == 0 {
		return nil, errors.New("no transactions to add")
	}

	gen := s.db().GetByID(req.SkipchainID)
	if gen == nil || gen.Index != 0 {
		return nil, errors.New("skipchain ID is does not exist")
	}

	s.txBuffer.add(string(req.SkipchainID), req.Transaction)

	if req.InclusionWait > 0 {
		// Wait for InclusionWait new blocks and look if our transaction is in it.
		interval, err := LoadBlockIntervalFromColl(s.GetCollectionView(req.SkipchainID))
		if err != nil {
			return nil, errors.New("couldn't get collectionView: " + err.Error())
		}
		ctxHash := req.Transaction.Instructions.Hash()
		ch := s.state.createWaitChannel(ctxHash)
		defer s.state.deleteWaitChannel(ctxHash)
		select {
		case success := <-ch:
			if !success {
				return nil, errors.New("transaction is in block, but got refused")
			}
		case <-time.After(time.Duration(req.InclusionWait) * interval):
			return nil, fmt.Errorf("didn't find transaction in %v blocks", req.InclusionWait)
		}
	}
	return &AddTxResponse{
		Version: CurrentVersion,
	}, nil
}

// GetProof searches for a key and returns a proof of the
// presence or the absence of this key.
func (s *Service) GetProof(req *GetProof) (resp *GetProofResponse, err error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}
	log.Lvlf2("%s: Getting proof for key %x on sc %x", s.ServerIdentity(), req.Key, req.ID)
	latest, err := s.db().GetLatestByID(req.ID)
	if err != nil && latest == nil {
		return
	}
	proof, err := NewProof(s.getCollection(req.ID), s.db(), latest.Hash, req.Key)
	if err != nil {
		return
	}
	resp = &GetProofResponse{
		Version: CurrentVersion,
		Proof:   *proof,
	}
	return
}

// SetPropagationTimeout overrides the default propagation timeout that is used
// when a new block is announced to the nodes as well as the skipchain
// propagation timeout.
func (s *Service) SetPropagationTimeout(p time.Duration) {
	s.storage.Lock()
	s.storage.PropTimeout = p
	s.storage.Unlock()
	s.save()
	s.skService().SetPropTimeout(p)
}

// createNewBlock creates a new block and proposes it to the
// skipchain-service. Once the block has been created, we
// inform all nodes to update their internal collections
// to include the new transactions.
func (s *Service) createNewBlock(scID skipchain.SkipBlockID, r *onet.Roster, tx []TxResult) (*skipchain.SkipBlock, error) {
	var sb *skipchain.SkipBlock
	var mr []byte
	var coll *collection.Collection

	if scID.IsNull() {
		// For a genesis block, we create a throwaway collection.
		// There is no need to verify the darc because the caller does
		// it.
		sb = skipchain.NewSkipBlock()
		sb.Roster = r
		sb.MaximumHeight = 10
		sb.BaseHeight = 10
		// We have to register the verification functions in the genesis block
		sb.VerifierIDs = []skipchain.VerifierID{skipchain.VerifyBase, verifyOmniLedger}

		coll = collection.New(&collection.Data{}, &collection.Data{}, &collection.Data{})
	} else {
		// For all other blocks, we try to verify the signature using
		// the darcs and remove those that do not have a valid
		// signature before continuing.
		sbLatest, err := s.db().GetLatestByID(scID)
		if err != nil {
			return nil, errors.New(
				"Could not get latest block from the skipchain: " + err.Error())
		}
		log.Lvlf3("Creating block #%d with %d transactions", sbLatest.Index+1,
			len(tx))
		sb = sbLatest.Copy()
		if r != nil {
			sb.Roster = r
		}

		coll = s.getCollection(scID).coll
	}

	// Create header of skipblock containing only hashes
	var scs StateChanges
	var err error
	var txRes TxResults

	log.Lvl3("Creating state changes")
	mr, txRes, scs = s.createStateChanges(coll, scID, tx, noTimeout)
	if len(txRes) == 0 {
		return nil, errors.New("no transactions")
	}

	// Store transactions in the body
	body := &DataBody{TxResults: txRes}
	sb.Payload, err = protobuf.Encode(body)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

	header := &DataHeader{
		CollectionRoot:        mr,
		ClientTransactionHash: txRes.Hash(),
		StateChangesHash:      scs.Hash(),
		Timestamp:             time.Now().UnixNano(),
	}
	sb.Data, err = protobuf.Encode(header)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

	var ssb = skipchain.StoreSkipBlock{
		NewBlock:          sb,
		TargetSkipChainID: scID,
	}
	log.Lvlf3("Storing skipblock with %d transactions.", len(txRes))
	ssbReply, err := s.skService().StoreSkipBlock(&ssb)
	if err != nil {
		return nil, err
	}
	return ssbReply.Latest, nil
}

// updateCollectionCallback is registered in skipchain and is called after a
// skipblock is updated. When this function is called, it is not always after
// the addition of a new block, but an updates to forward links, for example.
// Hence, we need to figure out when a new block is added. This can be done by
// looking at the latest skipblock cache from Service.state.
func (s *Service) updateCollectionCallback(sbID skipchain.SkipBlockID) error {
	log.Lvlf4("%s: callback on %x", s.ServerIdentity(), sbID)
	if !s.isOurChain(sbID) {
		log.Lvl4("Not our chain...")
		return nil
	}
	sb := s.db().GetByID(sbID)
	if sb == nil {
		panic("This should never happen because the callback runs " +
			"only after the skipblock is stored. There is a " +
			"programmer error if you see this message.")
	}
	prevSB := s.db().GetByID(s.state.getLast(sb.SkipChainID()))
	var prevIdx int
	if prevSB == nil {
		prevIdx = -1
	} else {
		prevIdx = prevSB.Index
	}
	if prevIdx+1 != sb.Index {
		log.Lvl4(s.ServerIdentity(), "not updating collection because it is not a new block")
		return nil
	}

	var header DataHeader
	err := protobuf.DecodeWithConstructors(sb.Data, &header, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		log.Error(s.ServerIdentity(), "could not unmarshal header", err)
		return errors.New("couldn't unmarshal header")
	}

	var body DataBody
	err = protobuf.DecodeWithConstructors(sb.Payload, &body, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		log.Error(s.ServerIdentity(), "could not unmarshal body", err)
		return errors.New("couldn't unmarshal body")
	}

	log.Lvlf2("%s: Updating transactions for %x", s.ServerIdentity(), sb.SkipChainID())
	cdb := s.getCollection(sb.SkipChainID())
	_, _, scs := s.createStateChanges(cdb.coll, sb.SkipChainID(), body.TxResults, noTimeout)

	log.Lvlf3("%s: Storing %d state changes %v", s.ServerIdentity(), len(scs), scs.ShortStrings())
	if err = cdb.StoreAll(scs); err != nil {
		return err
	}
	if !bytes.Equal(cdb.RootHash(), header.CollectionRoot) {
		log.Error("hash of collection doesn't correspond to root hash")
	}
	s.state.setLast(sb)

	// Notify all waiting channels
	for _, t := range body.TxResults {
		s.state.informWaitChannel(t.ClientTransaction.Instructions.Hash(), t.Accepted)
	}

	// check whether the heartbeat monitor exists, if it doesn't we start a
	// new one
	interval, err := s.LoadBlockInterval(sb.SkipChainID())
	if err != nil {
		return err
	}
	if s.heartbeats.enabled() && sb.Index == 0 {
		if s.heartbeats.exists(string(sb.SkipChainID())) {
			panic("This is a new genesis block, but we're already running " +
				"the heartbeat monitor, it should never happen.")
		}
		log.Lvlf2("%s: started heartbeat monitor for %x", s.ServerIdentity(), sb.SkipChainID())
		s.heartbeats.start(string(sb.SkipChainID()), interval*rotationWindow, s.heartbeatsTimeout)
	}

	// if we are the new leader, then start polling
	if sb.Roster.List[0].Equal(s.ServerIdentity()) {
		s.pollChanMut.Lock()
		if _, ok := s.pollChan[string(sb.SkipChainID())]; !ok {
			log.Lvlf2("%s: new leader started polling for %x", s.ServerIdentity(), sb.SkipChainID())
			s.pollChanWG.Add(1)
			s.pollChan[string(sb.SkipChainID())] = s.startPolling(sb.SkipChainID(), interval)
		}
		s.pollChanMut.Unlock()
	}

	// If we are adding a genesis block, then look into it for the darc ID
	// and add it to the darcToSc hash map.
	if sb.Index == 0 {
		// the information should already be in the collections
		d, err := s.LoadGenesisDarc(sb.SkipChainID())
		if err != nil {
			return err
		}
		s.darcToScMut.Lock()
		s.darcToSc[string(d.GetBaseID())] = sb.SkipChainID()
		s.darcToScMut.Unlock()
	}
	return nil
}

// GetCollectionView returns a read-only accessor to the collection
// for the given skipchain.
func (s *Service) GetCollectionView(scID skipchain.SkipBlockID) CollectionView {
	cdb := s.getCollection(scID)
	return &roCollection{cdb.coll}
}

func (s *Service) getCollection(id skipchain.SkipBlockID) *collectionDB {
	idStr := fmt.Sprintf("%x", id)
	col := s.collectionDB[idStr]
	if col == nil {
		db, name := s.GetAdditionalBucket([]byte(idStr))
		s.collectionDB[idStr] = newCollectionDB(db, name)
		return s.collectionDB[idStr]
	}
	return col
}

// interface to skipchain.Service
func (s *Service) skService() *skipchain.Service {
	return s.Service(skipchain.ServiceName).(*skipchain.Service)
}

// gives us access to the skipchain's database, so we can get blocks by ID
func (s *Service) db() *skipchain.SkipBlockDB {
	return s.skService().GetDB()
}

// LoadConfig loads the configuration from a skipchain ID.
func (s *Service) LoadConfig(scID skipchain.SkipBlockID) (*ChainConfig, error) {
	coll := s.GetCollectionView(scID)
	if coll == nil {
		return nil, errors.New("nil collection DB")
	}
	return LoadConfigFromColl(coll)
}

// LoadGenesisDarc loads the genesis darc of the given skipchain ID.
func (s *Service) LoadGenesisDarc(scID skipchain.SkipBlockID) (*darc.Darc, error) {
	coll := s.GetCollectionView(scID)
	return getInstanceDarc(coll, ConfigInstanceID)
}

// LoadBlockInterval loads the block interval from the skipchain ID.
func (s *Service) LoadBlockInterval(scID skipchain.SkipBlockID) (time.Duration, error) {
	collDb := s.getCollection(scID)
	if collDb == nil {
		return defaultInterval, errors.New("nil collection DB")
	}
	return LoadBlockIntervalFromColl(&roCollection{collDb.coll})
}

// EnableViewChange enables the view-change functionality. View-change is
// highly time sensitive, if the block interval is very low (e.g., when using
// tests), then we may see unexpected view-change requests while transactions
// are still being processed if it is enabled in tests.
func (s *Service) EnableViewChange() {
	s.skService().EnableViewChange()
	s.heartbeats = newHeartbeats()
	s.monitorLeaderFailure()
}

func (s *Service) startPolling(scID skipchain.SkipBlockID, interval time.Duration) chan bool {
	closeSignal := make(chan bool)
	go func() {
		defer s.pollChanWG.Done()
		var txs []ClientTransaction
		for {
			select {
			case <-closeSignal:
				log.Lvl2(s.ServerIdentity(), "stopping polling")
				return
			case <-time.After(interval):
				sb, err := s.db().GetLatestByID(scID)
				if err != nil {
					panic("DB is in bad state and cannot find skipchain anymore: " + err.Error() +
						" This function should never be called on a skipchain that does not exist.")
				}

				log.Lvl3("Starting new block", sb.Index+1)
				leader, err := s.getLeader(scID)
				if err != nil {
					panic("getLeader should not return an error if roster is initialised.")
				}
				if !leader.Equal(s.ServerIdentity()) {
					panic("startPolling should always be called by the leader," +
						" if it isn't, then it did not start or shutdown properly.")
				}
				tree := sb.Roster.GenerateNaryTree(len(sb.Roster.List))

				proto, err := s.CreateProtocol(collectTxProtocol, tree)
				if err != nil {
					panic("Protocol creation failed with error: " + err.Error() +
						" This panic indicates that there is most likely a programmer error," +
						" e.g., the protocol does not exist." +
						" Hence, we cannot recover from this failure without putting" +
						" the server in a strange state, so we panic.")
				}
				root := proto.(*CollectTxProtocol)
				root.SkipchainID = scID
				root.LatestID = sb.Hash
				if err := root.Start(); err != nil {
					panic("Failed to start the protocol with error: " + err.Error() +
						" Start() only returns an error when the protocol is not initialised correctly," +
						" e.g., not all the required fields are set." +
						" If you see this message then there may be a programmer error.")
				}

				// When we poll, the child nodes must reply within half of the block interval,
				// because we'll use the other half to process the transactions.
				protocolTimeout := time.After(interval / 2)
			collectTxLoop:
				for {
					select {
					case newTxs, more := <-root.TxsChan:
						if more {
							txs = append(txs, newTxs...)
						} else {
							break collectTxLoop
						}
					case <-protocolTimeout:
						log.Lvl2(s.ServerIdentity(), "timeout while collecting transactions from other nodes")
						close(root.Finish)
						break collectTxLoop
					case <-closeSignal:
						log.Lvl2(s.ServerIdentity(), "stopping polling")
						close(root.Finish)
						return
					}
				}
				log.Lvl3("Collected all new transactions:", len(txs))

				if len(txs) == 0 {
					log.Lvl3(s.ServerIdentity(), "no new transactions, not creating new block")
					continue
				}

				txIn := make([]TxResult, len(txs))
				for i := range txIn {
					txIn[i].ClientTransaction = txs[i]
				}

				// Pre-run transactions to look how many we can fit in the alloted time
				// slot. Perhaps we can run this in parallel during the wait-phase?
				log.Lvl3("Counting how many transactions fit in", interval/2)
				cdb := s.getCollection(scID)
				_, txOut, _ := s.createStateChanges(cdb.coll, scID, txIn, interval/2)

				txs = txs[len(txOut):]
				if len(txs) > 0 {
					log.Warnf("Got more transactions than can be done in half the blockInterval. "+
						"%d transactions left", len(txs))
				}

				_, err = s.createNewBlock(scID, sb.Roster, txOut)
				if err != nil {
					log.Error("couldn't create new block: " + err.Error())
				}
			}
		}
	}()
	return closeSignal
}

// We use the OmniLedger as a receiver (as is done in the identity service),
// so we can access e.g. the collectionDBs of the service.
func (s *Service) verifySkipBlock(newID []byte, newSB *skipchain.SkipBlock) bool {
	start := time.Now()
	defer func() {
		log.Lvlf3("%s: Verify done after %s", s.ServerIdentity(), time.Now().Sub(start))
	}()

	var header DataHeader
	err := protobuf.DecodeWithConstructors(newSB.Data, &header, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		log.Error("verifySkipblock: couldn't unmarshal header")
		return false
	}

	// Check the contents of the DataHeader before proceeding.
	// We'll check the timestamp later, once we have the config loaded.
	err = func() error {
		if len(header.CollectionRoot) != sha256.Size {
			return errors.New("collection root is wrong size")
		}
		if len(header.ClientTransactionHash) != sha256.Size {
			return errors.New("client transaction hash is wrong size")
		}
		if len(header.StateChangesHash) != sha256.Size {
			return errors.New("state changes hash is wrong size")
		}
		return nil
	}()

	if err != nil {
		log.Errorf("data header failed check: %v", err)
		return false
	}

	var body DataBody
	err = protobuf.DecodeWithConstructors(newSB.Payload, &body, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		log.Error("verifySkipblock: couldn't unmarshal body")
		return false
	}

	cdb := s.getCollection(newSB.SkipChainID())
	mtr, txOut, scs := s.createStateChanges(cdb.coll, newSB.SkipChainID(), body.TxResults, noTimeout)

	// Check that the locally generated list of accepted/rejected txs match the list
	// the leader proposed.
	if len(txOut) != len(body.TxResults) {
		log.Lvl2(s.ServerIdentity(), "transaction list length mismatch after execution")
		return false
	}
	for i := range txOut {
		if txOut[i].Accepted != body.TxResults[i].Accepted {
			log.Lvl2(s.ServerIdentity(), "Client Transaction accept mistmatch on tx", i)
			return false
		}
	}

	// Check that the hashes in DataHeader are right.
	if bytes.Compare(header.ClientTransactionHash, txOut.Hash()) != 0 {
		log.Lvl2(s.ServerIdentity(), "Client Transaction Hash doesn't verify")
		return false
	}

	if bytes.Compare(header.CollectionRoot, mtr) != 0 {
		log.Lvl2(s.ServerIdentity(), "Collection root doesn't verify")
		return false
	}
	if bytes.Compare(header.StateChangesHash, scs.Hash()) != 0 {
		log.Lvl2(s.ServerIdentity(), "State Changes hash doesn't verify")
		return false
	}

	// Compute the new state and check whether the roster in newSB matches
	// the config.
	collClone := s.getCollection(newSB.SkipChainID()).coll.Clone()
	for _, sc := range scs {
		if err := storeInColl(collClone, &sc); err != nil {
			log.Error(err)
			return false
		}
	}
	config, err := LoadConfigFromColl(&roCollection{collClone})
	if err != nil {
		log.Error(err)
		return false
	}
	if !config.Roster.ID.Equal(newSB.Roster.ID) {
		log.Error("rosters have unequal IDs")
		return false
	}
	for i := range config.Roster.List {
		if !newSB.Roster.List[i].Equal(config.Roster.List[i]) {
			log.Error("roster in config is not equal to the one in skipblock")
			return false
		}
	}

	now := time.Now()
	t1 := now.Add(-2 * config.BlockInterval)
	t2 := now.Add(2 * config.BlockInterval)
	ts := time.Unix(0, header.Timestamp)
	if ts.Before(t1) || ts.After(t2) {
		log.Errorf("timestamp %v is outside the acceptable range %v to %v", ts, t1, t2)
		return false
	}

	log.Lvl4(s.ServerIdentity(), "verification completed")
	return true
}

// createStateChanges goes through all the proposed transactions one by one,
// creating the appropriate StateChanges, by sorting out which transactions can
// be run, which fail, and which cannot be attempted yet (due to timeout).
//
// If timeout is not 0, createStateChanges will stop running instructions after
// that long, in order for the caller to determine how many instructions fit in
// a block interval.
//
// State caching is implemented here, which is critical to performance, because
// on the leader it reduces the number of contract executions by 1/3 and on
// followers by 1/2.
func (s *Service) createStateChanges(coll *collection.Collection, scID skipchain.SkipBlockID, txIn TxResults, timeout time.Duration) (merkleRoot []byte, txOut TxResults, states StateChanges) {
	// If what we want is in the cache, then take it from there. Otherwise
	// ignore the error and compute the state changes.
	var err error
	merkleRoot, txOut, states, err = s.stateChangeCache.get(scID, txIn.Hash())
	if err == nil {
		log.Lvl3(s.ServerIdentity(), "loaded state changes from cache")
		return
	}
	log.Lvl3(s.ServerIdentity(), "state changes from cache: MISS")
	err = nil

	deadline := time.Now().Add(timeout)

	// TODO: Because we depend on making at least one clone per transaction
	// we need to find out if this is as expensive as it looks, and if so if
	// we could use some kind of copy-on-write technique.

	cdbTemp := coll.Clone()
	var cin []Coin
clientTransactions:
	for _, tx := range txIn {
		// Make a new collection for each instruction. If the instruction is sucessfully
		// implemented and changes applied, then keep it (via cdbTemp = cdbI.c),
		// otherwise dump it.
		cdbI := &roCollection{cdbTemp.Clone()}
		for _, instr := range tx.ClientTransaction.Instructions {
			scs, cout, err := s.executeInstruction(cdbI, cin, instr)
			if err != nil {
				log.Errorf("%s: Call to contract returned error: %s", s.ServerIdentity(), err)
				tx.Accepted = false
				txOut = append(txOut, tx)
				continue clientTransactions
			}
			for _, sc := range scs {
				if err := storeInColl(cdbI.c, &sc); err != nil {
					log.Error("failed to add to collections with error: " + err.Error())
					tx.Accepted = false
					txOut = append(txOut, tx)
					continue clientTransactions
				}
			}
			states = append(states, scs...)
			cin = cout

		}
		// timeout is ONLY used when the leader calls createStateChanges as
		// part of planning which ClientTransactions fit into one block.
		if timeout != noTimeout {
			if time.Now().After(deadline) {
				return
			}
		}
		cdbTemp = cdbI.c
		tx.Accepted = true
		txOut = append(txOut, tx)
	}

	// Store the result in the cache before returning.
	merkleRoot = cdbTemp.GetRoot()
	s.stateChangeCache.update(scID, txOut.Hash(), merkleRoot, txOut, states)
	return
}

func (s *Service) executeInstruction(cdbI CollectionView, cin []Coin, instr Instruction) (scs StateChanges, cout []Coin, err error) {
	defer func() {
		if re := recover(); re != nil {
			err = errors.New(re.(string))
		}
	}()

	contractID, _, err := instr.GetContractState(cdbI)
	if err != nil {
		err = errors.New("Couldn't get contract type of instruction: " + err.Error())
		return
	}

	contract, exists := s.contracts[contractID]
	// If the leader does not have a verifier for this contract, it drops the
	// transaction.
	if !exists {
		err = errors.New("Leader is dropping instruction of unknown contract: " + contractID)
		return
	}
	// Now we call the contract function with the data of the key.
	log.Lvlf3("%s: Calling contract %s", s.ServerIdentity(), contractID)
	return contract(cdbI, instr, cin)
}

func (s *Service) getLeader(scID skipchain.SkipBlockID) (*network.ServerIdentity, error) {
	sb, err := s.db().GetLatestByID(scID)
	if err != nil {
		return nil, err
	}
	if sb.Roster == nil || len(sb.Roster.List) < 1 {
		return nil, errors.New("roster is empty")
	}
	return sb.Roster.List[0], nil
}

// getTxs is primarily used as a callback in the CollectTx protocol to retrieve
// a set of pending transactions. However, it is a very useful way to piggy
// back additional functionalities that need to be executed at every interval,
// such as updating the heartbeat monitor and synchronising the state.
func (s *Service) getTxs(leader *network.ServerIdentity, roster *onet.Roster, scID skipchain.SkipBlockID, latestID skipchain.SkipBlockID) []ClientTransaction {
	actualLeader, err := s.getLeader(scID)
	if err != nil {
		log.Lvlf1("could not find a leader on %x with error %s", scID, err)
		return []ClientTransaction{}
	}
	if !leader.Equal(actualLeader) {
		log.Warn(s.ServerIdentity(), "getTxs came from a wrong leader")
		return []ClientTransaction{}
	}
	if s.heartbeats.enabled() {
		s.heartbeats.beat(string(scID))
	}

	// If the leader's latestID is something we do not know about, then we
	// need to synchronise.
	// NOTE: there is a potential denial of service when the leader sends
	// an invalid latestID, but our current implementation assumes that the
	// leader cannot be byzantine (i.e., it can only exhibit crash
	// failure).
	ourLatest, err := s.db().GetLatestByID(scID)
	if err != nil {
		log.Warn(s.ServerIdentity(), "we do not know about the skipchain ID")
		return []ClientTransaction{}
	}
	latestSB := s.db().GetByID(latestID)
	if latestSB == nil {
		log.Lvl3(s.ServerIdentity(), "chain is out of date")
		if err := s.skService().SyncChain(roster, ourLatest.Hash); err != nil {
			log.Error(s.ServerIdentity(), err)
		}
	} else {
		log.Lvl3(s.ServerIdentity(), "chain is up to date")
	}

	return s.txBuffer.take(string(scID))
}

// TestClose closes the go-routines that are polling for transactions. It is
// exported because we need it in tests, it should not be used in non-test code
// outside of this package.
func (s *Service) TestClose() {
	s.pollChanMut.Lock()
	for k, c := range s.pollChan {
		close(c)
		delete(s.pollChan, k)
	}
	s.pollChanMut.Unlock()
	if s.heartbeats.enabled() {
		s.heartbeats.closeAll()
		s.heartbeatsClose <- true
	}

	s.pollChanWG.Wait()
}

func (s *Service) monitorLeaderFailure() {
	go func() {
		// Here we empty the messages in heartbeatsClose. This is
		// needed because tests may try to close the heartbeat monitor
		// multiple times. Further, if there's already something in the
		// close channel, and then we try to start the heartbeat
		// monitor, it will close immediately which would cause
		// confussion.
	emptyMessagesLabel:
		for {
			select {
			case <-s.heartbeatsClose:
			default:
				break emptyMessagesLabel
			}
		}
		for {
			select {
			case key := <-s.heartbeatsTimeout:
				log.Lvlf2("%s: heartbeat timeout at %d for %x", s.ServerIdentity(), time.Now().Unix(), []byte(key))
				scID := []byte(key)
				if err := s.startViewChange(scID); err != nil {
					log.Error(s.ServerIdentity(), err)
				}
			case <-s.heartbeatsClose:
				log.Lvl2(s.ServerIdentity(), "closing heartbeat timeout monitor")
				return
			}
		}
	}()
}

func (s *Service) startViewChange(scID skipchain.SkipBlockID) error {
	sb, err := s.db().GetLatestByID(scID)
	if err != nil {
		return err
	}
	if len(sb.Roster.List) < 2 {
		return errors.New("roster size is too small")
	}
	if !sb.Roster.List[1].Equal(s.ServerIdentity()) {
		// i'm not the next leader, do nothing
		return nil
	}

	_, _, genDarcID, err := s.GetCollectionView(scID).GetValues(NewInstanceID(nil).Slice())
	if err != nil {
		return err
	}

	newRoster := onet.NewRoster(append(sb.Roster.List[1:], sb.Roster.List[0]))
	newRosterBuf, err := protobuf.Encode(newRoster)
	if err != nil {
		return err
	}

	ctx := ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: NewInstanceID(nil),
			Nonce:      GenNonce(),
			Index:      0,
			Length:     1,
			Invoke: &Invoke{
				Command: "view_change",
				Args: []Argument{{
					Name:  "roster",
					Value: newRosterBuf,
				}},
			},
		}},
	}
	signer := darc.NewSignerEd25519(s.ServerIdentity().Public, s.getPrivateKey())
	if err = ctx.Instructions[0].SignBy(genDarcID, signer); err != nil {
		return err
	}

	log.Lvlf2("%s: proposing view-change for %x", s.ServerIdentity(), scID)
	_, err = s.createNewBlock(scID, newRoster, []TxResult{{ClientTransaction: ctx}})
	return err
}

// getPrivateKey is a hack that creates a temporary TreeNodeInstance and gets
// the private key out of it. We have to do this because we cannot access the
// private key from the service.
func (s *Service) getPrivateKey() kyber.Scalar {
	tree := onet.NewRoster([]*network.ServerIdentity{s.ServerIdentity()}).GenerateBinaryTree()
	tni := s.NewTreeNodeInstance(tree, tree.Root, "dummy")
	return tni.Private()
}

func (s *Service) scIDFromGenesisDarc(genesisDarcID darc.ID) (skipchain.SkipBlockID, error) {
	s.darcToScMut.Lock()
	scID, ok := s.darcToSc[string(genesisDarcID)]
	s.darcToScMut.Unlock()
	if !ok {
		return nil, errors.New("the specified genesis darc ID does not exist")
	}
	return scID, nil
}

// withinInterval checks whether public key targetPk in skipchain that has the
// genesis darc genesisDarcID has the right to be the new leader at the current
// time. This function should only be called when view-change is enabled.
func (s *Service) withinInterval(genesisDarcID darc.ID, targetPk kyber.Point) error {
	scID, err := s.scIDFromGenesisDarc(genesisDarcID)
	if err != nil {
		return err
	}

	interval, err := s.LoadBlockInterval(scID)
	if err != nil {
		return err
	}
	t, err := s.heartbeats.getLatestHeartbeat(string(scID))
	if err != nil {
		return err
	}

	// After a leader dies, good nodes wait for 2*interval before accepting
	// new leader proposals. For every time window of 2*interval
	// afterwards, the "next" node in the roster list has a chance to
	// propose to be a new leader.

	currTime := time.Now()
	if t.Add(time.Duration(rotationWindow) * interval).After(currTime) {
		return errors.New("not ready to accept new leader yet")
	}

	// find the position in the proposer queue
	latestConfig, err := s.LoadConfig(scID)
	if err != nil {
		return err
	}
	pos := func() int {
		var ctr int
		for _, pk := range latestConfig.Roster.Publics() {
			if pk.Equal(targetPk) {
				return ctr
			}
			ctr++
		}
		return -1
	}()
	if pos == -1 || pos == 0 {
		return errors.New("invalid targetPk " + targetPk.String() + ", or position " + string(pos))
	}

	// check that the time window matches with the position using the
	// equation below, note that t = previous heartbeat
	// t + pos * 2 * interval < now < t + (pos+1) * 2 * interval
	tLower := t.Add(time.Duration(pos) * rotationWindow * interval)
	tUpper := t.Add(time.Duration(pos+1) * rotationWindow * interval)
	if currTime.After(tLower) && currTime.Before(tUpper) {
		return nil
	}
	return errors.New("not your turn to change leader")
}

// registerContract stores the contract in a map and will
// call it whenever a contract needs to be done.
func (s *Service) registerContract(contractID string, c OmniLedgerContract) error {
	s.contracts[contractID] = c
	return nil
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.SetPropagationTimeout(120 * time.Second)

	msg, err := s.Load(storageID)
	if err != nil {
		return err
	}
	if msg != nil {
		var ok bool
		s.storage, ok = msg.(*omniStorage)
		if !ok {
			return errors.New("Data of wrong type")
		}
	}
	s.collectionDB = map[string]*collectionDB{}
	s.state = olState{
		lastBlock:    make(map[string]skipchain.SkipBlockID),
		waitChannels: make(map[string]chan bool),
	}

	// NOTE: Usually tryLoad is only called when services start up. but for
	// testing, we might re-initialise the service. So we need to clean up
	// the go-routines.
	s.TestClose()

	// Recreate the polling channles.
	s.pollChanMut.Lock()
	defer s.pollChanMut.Unlock()
	s.pollChan = make(map[string]chan bool)

	gas := &skipchain.GetAllSkipChainIDs{}
	gasr, err := s.skService().GetAllSkipChainIDs(gas)
	if err != nil {
		return err
	}

	for _, gen := range gasr.IDs {
		if !s.isOurChain(gen) {
			continue
		}
		interval, err := s.LoadBlockInterval(gen)
		if err != nil {
			log.Errorf("Ignoring chain %x because we can't load blockInterval: %s", gen, err)
			continue
		}

		leader, err := s.getLeader(gen)
		if err != nil {
			panic("getLeader should not return an error if roster is initialised.")
		}
		if leader.Equal(s.ServerIdentity()) {
			s.pollChanWG.Add(1)
			s.pollChan[string(gen)] = s.startPolling(gen, interval)
		}
		sb, err := s.db().GetLatestByID(gen)
		if err != nil {
			return err
		}
		s.state.setLast(sb)

		// populate the darcID to skipchainID mapping
		d, err := s.LoadGenesisDarc(gen)
		if err != nil {
			return err
		}
		s.darcToScMut.Lock()
		s.darcToSc[string(d.GetBaseID())] = gen
		s.darcToScMut.Unlock()
	}

	return nil
}

// checks that a given chain has a verifier we recognize
func (s *Service) isOurChain(gen skipchain.SkipBlockID) bool {
	sb := s.db().GetByID(gen)
	if sb == nil {
		// Not finding this ID should not happen, but
		// if it does, just say "not ours".
		return false
	}
	for _, x := range sb.VerifierIDs {
		if x.Equal(verifyOmniLedger) {
			return true
		}
	}
	return false
}

// saves this service's config information
func (s *Service) save() {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save(storageID, s.storage)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real
// deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor:  onet.NewServiceProcessor(c),
		contracts:         make(map[string]OmniLedgerContract),
		txBuffer:          newTxBuffer(),
		heartbeatsTimeout: make(chan string, 1),
		heartbeatsClose:   make(chan bool, 1),
		storage:           &omniStorage{},
		darcToSc:          make(map[string]skipchain.SkipBlockID),
		stateChangeCache:  newStateChangeCache(),
	}
	if err := s.RegisterHandlers(s.CreateGenesisBlock, s.AddTransaction,
		s.GetProof); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}

	s.registerContract(ContractConfigID, s.ContractConfig)
	s.registerContract(ContractDarcID, s.ContractDarc)
	skipchain.RegisterVerification(c, verifyOmniLedger, s.verifySkipBlock)
	if _, err := s.ProtocolRegister(collectTxProtocol, NewCollectTxProtocol(s.getTxs)); err != nil {
		return nil, err
	}
	s.skService().RegisterStoreSkipblockCallback(s.updateCollectionCallback)
	return s, nil
}

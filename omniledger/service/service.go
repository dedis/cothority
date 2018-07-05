// Package service implements the OmniLedger service.
package service

import (
	"bytes"
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
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/satori/go.uuid.v1"
)

const darcIDLen int = 32

const invokeEvolve darc.Action = darc.Action("invoke:evolve")

// OmniledgerID can be used to refer to this service
var OmniledgerID onet.ServiceID

const collectTxProtocol = "CollectTxProtocol"

var verifyOmniLedger = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "OmniLedger"))

func init() {
	var err error
	OmniledgerID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &DataHeader{}, &updateCollection{})
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

	// pollChan maintains a map of channels that can be used to stop the
	// polling go-routing.
	pollChan    map[string]chan bool
	pollChanMut sync.Mutex

	// NOTE: If we have a lot of skipchains, then using mutex most likely
	// will slow down our service, an improvement is to go-routines to
	// store transactions. But there is more management overhead, e.g.,
	// restarting after shutdown, answer getTxs requests and so on.
	txBuffer txBuffer

	// contracts map kinds to kind specific verification functions
	contracts map[string]OmniLedgerContract
	// propagate the new transactions
	propagateTransactions messaging.PropagationFunc

	storage *storage

	createSkipChainMut sync.Mutex
}

// storageID reflects the data we're storing - we could store more
// than one structure.
const storageID = "main"

// defaultInterval is used if the BlockInterval field in the genesis
// transaction is not set.
var defaultInterval = 5 * time.Second

// storage is used to save our data locally.
type storage struct {
	sync.Mutex
	// PropTimeout is used when sending the request to integrate a new block
	// to all nodes.
	PropTimeout time.Duration
}

type updateCollection struct {
	ID skipchain.SkipBlockID
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
		len(req.GenesisDarc.Rules) == 0 {
		return nil, errors.New("invalid genesis darc")
	}

	if req.BlockInterval == 0 {
		req.BlockInterval = defaultInterval
	}
	intervalBuf := make([]byte, 8)
	binary.PutVarint(intervalBuf, int64(req.BlockInterval))

	spawn := &Spawn{
		ContractID: ContractConfigID,
		Args: Arguments{
			{Name: "darc", Value: darcBuf},
			{Name: "block_interval", Value: intervalBuf},
		},
	}

	// Create the genesis-transaction with a special key, it acts as a
	// reference to the actual genesis transaction.
	transaction := []ClientTransaction{{
		Instructions: []Instruction{{
			InstanceID: InstanceID{DarcID: req.GenesisDarc.GetID()},
			Nonce:      Nonce{},
			Index:      0,
			Length:     1,
			Spawn:      spawn,
		}},
	}}

	sb, err := s.createNewBlock(nil, &req.Roster, transaction)
	if err != nil {
		return nil, err
	}

	s.pollChanMut.Lock()
	s.pollChan[string(sb.SkipChainID())] = s.startPolling(sb.SkipChainID(), req.BlockInterval)
	s.pollChanMut.Unlock()

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
// when a new block is announced to the nodes.
func (s *Service) SetPropagationTimeout(p time.Duration) {
	s.storage.Lock()
	s.storage.PropTimeout = p
	s.storage.Unlock()
	s.save()
}

func toInstanceID(dID darc.ID) InstanceID {
	return InstanceID{
		DarcID: dID,
		SubID:  SubID{},
	}
}

func (s *Service) verifyAndFilterTxs(scID skipchain.SkipBlockID, ts []ClientTransaction) []ClientTransaction {
	var validTxs []ClientTransaction
	for _, t := range ts {
		if err := s.verifyClientTx(scID, t); err != nil {
			log.Error(err)
			continue
		}
		validTxs = append(validTxs, t)
	}
	return validTxs
}

func (s *Service) verifyClientTx(scID skipchain.SkipBlockID, tx ClientTransaction) error {
	for _, instr := range tx.Instructions {
		if err := s.verifyInstruction(scID, instr); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) verifyInstruction(scID skipchain.SkipBlockID, instr Instruction) error {
	d, err := s.loadLatestDarc(scID, instr.InstanceID.DarcID)
	if err != nil {
		return errors.New("darc not found: " + err.Error())
	}
	req, err := instr.ToDarcRequest()
	if err != nil {
		return errors.New("couldn't create darc request: " + err.Error())
	}
	// TODO we need to use req.VerifyWithCB to search for missing darcs
	err = req.Verify(d)
	if err != nil {
		return errors.New("request verification failed: " + err.Error())
	}
	return nil
}

// createNewBlock creates a new block and proposes it to the
// skipchain-service. Once the block has been created, we
// inform all nodes to update their internal collections
// to include the new transactions.
func (s *Service) createNewBlock(scID skipchain.SkipBlockID, r *onet.Roster, cts ClientTransactions) (*skipchain.SkipBlock, error) {
	var sb *skipchain.SkipBlock
	var mr []byte
	var coll collection.Collection

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

		coll = collection.New(&collection.Data{}, &collection.Data{})
	} else {
		// For all other blocks, we try to verify the signature using
		// the darcs and remove those that do not have a valid
		// signature before continuing.
		sbLatest, err := s.db().GetLatest(s.db().GetByID(scID))
		if err != nil {
			return nil, errors.New(
				"Could not get latest block from the skipchain: " + err.Error())
		}
		sb = sbLatest.Copy()
		if r != nil {
			sb.Roster = r
		}
		cts = s.verifyAndFilterTxs(sb.SkipChainID(), cts)
		if len(cts) == 0 {
			return nil, errors.New("no valid transaction")
		}
		coll = s.getCollection(scID).coll
	}

	// Note that the transactions are sorted in-place.
	if err := sortTransactions(cts); err != nil {
		return nil, err
	}

	// Create header of skipblock containing only hashes
	var scs StateChanges
	var err error
	var ctsOK ClientTransactions
	mr, ctsOK, scs, err = s.createStateChanges(coll, cts)
	if err != nil {
		return nil, err
	}
	if len(scs) == 0 {
		return nil, errors.New("no state changes")
	}
	header := &DataHeader{
		CollectionRoot:        mr,
		ClientTransactionHash: ctsOK.Hash(),
		StateChangesHash:      scs.Hash(),
		Timestamp:             time.Now().Unix(),
	}
	sb.Data, err = network.Marshal(header)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

	// Store transactions in the body
	body := &DataBody{Transactions: ctsOK}
	sb.Payload, err = network.Marshal(body)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

	var ssb = skipchain.StoreSkipBlock{
		NewBlock:          sb,
		TargetSkipChainID: scID,
	}
	log.Lvlf2("Storing skipblock with transactions %+v", ctsOK)
	ssbReply, err := s.skService().StoreSkipBlock(&ssb)
	if err != nil {
		return nil, err
	}

	s.storage.Lock()
	pto := s.storage.PropTimeout
	s.storage.Unlock()
	// TODO: replace this with some kind of callback from the skipchain-service
	replies, err := s.propagateTransactions(sb.Roster, &updateCollection{sb.Hash}, pto)
	if err != nil {
		log.Lvl1("Propagation-error:", err.Error())
	}
	if replies != len(sb.Roster.List) {
		log.Lvl1(s.ServerIdentity(), "Only got", replies, "out of", len(sb.Roster.List))
	}

	return ssbReply.Latest, nil
}

// updateCollection is called once a skipblock has been stored.
// It is called by the leader, and every node will add the
// transactions in the block to its collection.
func (s *Service) updateCollection(msg network.Message) {
	uc, ok := msg.(*updateCollection)
	if !ok {
		log.Error("updateCollection cast failure")
		return
	}

	sb, err := s.db().GetLatestByID(uc.ID)
	if err != nil {
		log.Errorf("didn't find latest block for %x", uc.ID)
		return
	}
	_, dataI, err := network.Unmarshal(sb.Data, cothority.Suite)
	data, ok := dataI.(*DataHeader)
	if err != nil || !ok {
		log.Error("couldn't unmarshal header")
		return
	}
	_, bodyI, err := network.Unmarshal(sb.Payload, cothority.Suite)
	body, ok := bodyI.(*DataBody)
	if err != nil || !ok {
		log.Error("couldn't unmarshal body", err, ok)
		return
	}

	log.Lvlf2("%s: Updating transactions for %x", s.ServerIdentity(), sb.SkipChainID())
	cdb := s.getCollection(sb.SkipChainID())
	_, _, scs, err := s.createStateChanges(cdb.coll, body.Transactions)
	if err != nil {
		log.Error("Couldn't recreate state changes:", err.Error())
		return
	}

	if i, _ := sb.Roster.Search(s.ServerIdentity().ID); i == 0 {
		log.Lvlf2("%s: Storing state changes %v", s.ServerIdentity(), scs.ShortStrings())
	} else {
		log.Lvlf3("%s: Storing state changes %v", s.ServerIdentity(), scs.ShortStrings())
	}
	for _, sc := range scs {
		err = cdb.Store(&sc)
		if err != nil {
			log.Error("error while storing in collection: " + err.Error())
		}
	}
	if !bytes.Equal(cdb.RootHash(), data.CollectionRoot) {
		log.Error("hash of collection doesn't correspond to root hash")
	}
}

// GetCollectionView returns a read-only accessor to the collection
// for the given skipchain.
func (s *Service) GetCollectionView(id skipchain.SkipBlockID) CollectionView {
	cdb := s.getCollection(id)
	return &roCollection{c: cdb.coll}
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
	collDb := s.getCollection(scID)
	if collDb == nil {
		return nil, errors.New("nil collection DB")
	}
	return LoadConfigFromColl(&roCollection{collDb.coll})
}

// LoadBlockInterval loads the block interval from the skipchain ID.
func (s *Service) LoadBlockInterval(scID skipchain.SkipBlockID) (time.Duration, error) {
	collDb := s.getCollection(scID)
	if collDb == nil {
		return defaultInterval, errors.New("nil collection DB")
	}
	return LoadBlockIntervalFromColl(&roCollection{collDb.coll})
}

func (s *Service) loadLatestDarc(sid skipchain.SkipBlockID, dID darc.ID) (*darc.Darc, error) {
	colldb := s.getCollection(sid)
	if colldb == nil {
		return nil, fmt.Errorf("collection for skipchain ID %s does not exist", sid.Short())
	}
	value, contract, err := colldb.GetValueContract(toInstanceID(dID).Slice())
	if err != nil {
		return nil, err
	}
	if string(contract) != "darc" {
		return nil, fmt.Errorf("for darc %x, expected Kind to be 'darc' but got '%v'", dID, string(contract))
	}
	return darc.NewFromProtobuf(value)
}

func (s *Service) startPolling(scID skipchain.SkipBlockID, interval time.Duration) chan bool {
	closeSignal := make(chan bool)
	go func() {
		for {
			select {
			case <-time.After(interval):
				sb, err := s.db().GetLatestByID(scID)
				if err != nil {
					panic("DB is in bad state and cannot find skipchain anymore: " + err.Error() +
						" This function should never be called on a skipchain that does not exist.")
				}
				// We assume the caller of this function is the
				// current leader. So we generate a tree with
				// this node as the root.
				tree := sb.Roster.GenerateNaryTreeWithRoot(len(sb.Roster.List), s.ServerIdentity())
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
				if err := root.Start(); err != nil {
					panic("Failed to start the protocol with error: " + err.Error() +
						" Start() only returns an error when the protocol is not initialised correctly," +
						" e.g., not all the required fields are set." +
						" If you see this message then there may be a programmer error.")
				}

				protocolTimeout := time.After(s.skService().GetPropTimeout())
				var txs ClientTransactions
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
						break collectTxLoop
					case <-closeSignal:
						log.Lvl2(s.ServerIdentity(), "stopping polling")
						return
					}
				}

				if txs.IsEmpty() {
					log.Lvl3(s.ServerIdentity(), "no new transactions, not creating new block")
					continue
				}

				_, err = s.createNewBlock(scID, sb.Roster, txs)
				if err != nil {
					log.Error("couldn't create new block: " + err.Error())
				}
			case <-closeSignal:
				log.Lvl2(s.ServerIdentity(), "stopping polling")
				return
			}
		}
	}()
	return closeSignal
}

// We use the OmniLedger as a receiver (as is done in the identity service),
// so we can access e.g. the collectionDBs of the service.
func (s *Service) verifySkipBlock(newID []byte, newSB *skipchain.SkipBlock) bool {
	_, headerI, err := network.Unmarshal(newSB.Data, cothority.Suite)
	header, ok := headerI.(*DataHeader)
	if err != nil || !ok {
		log.Errorf("couldn't unmarshal header")
		return false
	}
	_, bodyI, err := network.Unmarshal(newSB.Payload, cothority.Suite)
	body, ok := bodyI.(*DataBody)
	if err != nil || !ok {
		log.Error("couldn't unmarshal body", err, ok)
		return false
	}

	if bytes.Compare(header.ClientTransactionHash, body.Transactions.Hash()) != 0 {
		log.Lvl2(s.ServerIdentity(), "Client Transaction Hash doesn't verify")
		return false
	}
	ctx := body.Transactions
	cdb := s.getCollection(newSB.SkipChainID())
	mtr, _, scs, err := s.createStateChanges(cdb.coll, ctx)
	if err != nil {
		log.Error("Couldn't create state changes:", err)
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
	return true
}

// createStateChanges goes through all ClientTransactions and creates
// the appropriate StateChanges. If any of the transactions are invalid,
// it returns an error.
func (s *Service) createStateChanges(coll collection.Collection, cts ClientTransactions) (merkleRoot []byte, ctsOK ClientTransactions, states StateChanges, err error) {

	// TODO: Because we depend on making at least one clone per transaction
	// we need to find out if this is as expensive as it looks, and if so if
	// we could use some kind of copy-on-write technique.

	cdbTemp := coll.Clone()
clientTransactions:
	for _, ct := range cts {
		// Make a new collection for each instruction. If the instruction is sucessfully
		// implemented and changes applied, then keep it (via cdbTemp = cdbI.c),
		// otherwise dump it.
		cdbI := &roCollection{c: cdbTemp.Clone()}
		for _, instr := range ct.Instructions {
			contract, _, err := instr.GetContractState(cdbI)
			if err != nil {
				log.Error("Couldn't get contract type of instruction:", err)
				continue clientTransactions
			}

			f, exists := s.contracts[contract]
			// If the leader does not have a verifier for this contract, it drops the
			// transaction.
			if !exists {
				log.Error("Leader is dropping instruction of unknown contract:", contract)
				continue clientTransactions
			}
			// Now we call the contract function with the data of the key.
			// Wrap up f() inside of g(), so that we can recover panics
			// from f().
			log.Lvlf3("%s: Calling contract %s", s.ServerIdentity(), contract)
			g := func(cdb CollectionView, inst Instruction, c []Coin) (sc []StateChange, cout []Coin, err error) {
				defer func() {
					if re := recover(); re != nil {
						err = errors.New(re.(string))
					}
				}()
				sc, cout, err = f(cdb, inst, c)
				return
			}
			scs, _, err := g(cdbI, instr, nil)
			if err != nil {
				log.Error("Call to contract returned error:", err)
				continue clientTransactions
			}
			for _, sc := range scs {
				if err := storeInColl(cdbI.c, &sc); err != nil {
					log.Error("failed to add to collections with error: " + err.Error())
					continue clientTransactions
				}
			}
			states = append(states, scs...)
		}
		cdbTemp = cdbI.c
		ctsOK = append(ctsOK, ct)
	}
	return cdbTemp.GetRoot(), ctsOK, states, nil
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

func (s *Service) getTxs(leader *network.ServerIdentity, scID skipchain.SkipBlockID) ClientTransactions {
	actualLeader, err := s.getLeader(scID)
	if err != nil {
		log.Lvlf1("could not find a leader on %x with error %s", scID, err)
		return []ClientTransaction{}
	}
	if !leader.Equal(actualLeader) {
		log.Lvl1("getTxs came from a wrong leader")
		return []ClientTransaction{}
	}
	return s.txBuffer.take(string(scID))
}

// ClosePolling closes the go-routines that are polling for transactions.
func (s *Service) ClosePolling() {
	for _, c := range s.pollChan {
		close(c)
	}
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
	s.storage = &storage{
		PropTimeout: 10 * time.Second,
	}
	msg, err := s.Load([]byte(storageID))
	if err != nil {
		return err
	}
	if msg != nil {
		var ok bool
		s.storage, ok = msg.(*storage)
		if !ok {
			return errors.New("Data of wrong type")
		}
	}
	if s.storage == nil {
		s.storage = &storage{}
	}
	s.collectionDB = map[string]*collectionDB{}

	// NOTE: Usually tryLoad is only called when services start up. but for
	// testing, we might re-initialise the service. So we need to clean up
	// the go-routines.
	s.pollChanMut.Lock()
	defer s.pollChanMut.Unlock()
	s.ClosePolling()
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
			return err
		}
		s.pollChan[string(gen)] = s.startPolling(gen, interval)
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
	err := s.Save([]byte(storageID), s.storage)
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
		ServiceProcessor: onet.NewServiceProcessor(c),
		contracts:        make(map[string]OmniLedgerContract),
		txBuffer:         newTxBuffer(),
	}
	if err := s.RegisterHandlers(s.CreateGenesisBlock, s.AddTransaction,
		s.GetProof); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}

	var err error
	s.propagateTransactions, err = messaging.NewPropagationFunc(c, "OmniLedgerPropagate", s.updateCollection, -1)
	if err != nil {
		return nil, err
	}

	s.registerContract(ContractConfigID, s.ContractConfig)
	s.registerContract(ContractDarcID, s.ContractDarc)
	skipchain.RegisterVerification(c, verifyOmniLedger, s.verifySkipBlock)
	if _, err := s.ProtocolRegister(collectTxProtocol, NewCollectTxProtocol(s.getTxs)); err != nil {
		return nil, err
	}
	return s, nil
}

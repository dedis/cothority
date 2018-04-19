// Package service implements the lleap service using the collection library to
// handle the merkle-tree. Each call to SetKeyValue updates the Merkle-tree and
// creates a new block containing the root of the Merkle-tree plus the new
// value that has been stored last in the Merkle-tree.
package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/util/key"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
)

// Used for tests
var lleapID onet.ServiceID

const keyMerkleRoot = "merkleroot"
const keyNewKey = "newkey"
const keyNewValue = "newvalue"

func init() {
	var err error
	lleapID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &Data{})
}

// Service is our lleap-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	// collections cannot be stored, so they will be re-created whenever the
	// service reloads.
	collectionDB map[string]*collectionDB

	storage *storage
}

// storageID reflects the data we're storing - we could store more
// than one structure.
const storageID = "main"

// Data is the data passed to the Skipchain
type Data struct {
	// Root of the merkle tree after applying the transactions to the
	// kv store
	MerkleRoot []byte
	// The transactions applied to the kv store with this block
	Transactions []*Transaction
	Timestamp    int64
	Roster       *onet.Roster
}

// storage is used to save our data locally.
type storage struct {
	// PL: Is used to sign the votes
	Private map[string]kyber.Scalar
	sync.Mutex
}

// CreateSkipchain asks the cisc-service to create a new skipchain ready to
// store key/value pairs. If it is given exactly one writer, this writer will
// be stored in the skipchain.
// For faster access, all data is also stored locally in the Service.storage
// structure.
func (s *Service) CreateSkipchain(req *CreateSkipchain) (
	*CreateSkipchainResponse, error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	kp := key.NewKeyPair(cothority.Suite)

	tmpColl := collection.New(collection.Data{}, collection.Data{})
	sigBuf, err := network.Marshal(&req.Transaction.Signature)
	if err != nil {
		return nil, errors.New("Couldn't marshal Signature: " + err.Error())
	}
	err = tmpColl.Add(req.Transaction.Key, req.Transaction.Value, sigBuf)
	if err != nil {
		return nil, errors.New("error while storing in collection: " + err.Error())
	}

	mr := tmpColl.GetRoot()
	data := &Data{
		MerkleRoot:   mr,
		Transactions: []*Transaction{&req.Transaction},
		Timestamp:    time.Now().Unix(),
	}

	buf, err := network.Marshal(data)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

	var genesisBlock = skipchain.NewSkipBlock()
	genesisBlock.Data = buf
	genesisBlock.Roster = &req.Roster
	genesisBlock.MaximumHeight = 1
	genesisBlock.BaseHeight = 1

	// TODO: Signature?
	var ssb = skipchain.StoreSkipBlock{NewBlock: genesisBlock}
	ssbReply, err := s.skService().StoreSkipBlock(&ssb)
	if err != nil {
		return nil, err
	}
	skID := ssbReply.Latest.SkipChainID()
	gid := string(skID)

	err = s.getCollection(skID).Store(req.Transaction.Key, req.Transaction.Value, sigBuf)
	if err != nil {
		return nil, errors.New(
			"error while storing in collection: " + err.Error())
	}
	s.storage.Private[gid] = kp.Private
	s.save()
	return &CreateSkipchainResponse{
		Version:   CurrentVersion,
		Skipblock: ssbReply.Latest,
	}, nil
}

// SetKeyValue asks cisc to add a new key/value pair.
func (s *Service) SetKeyValue(req *SetKeyValue) (*SetKeyValueResponse, error) {
	// Check the input arguments
	// TODO: verify the signature on the key/value pair
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}
	gid := string(req.SkipchainID)
	latest, err := s.db().GetLatest(s.db().GetByID(req.SkipchainID))
	if err != nil {
		return nil, errors.New(
			"Could not get latest block from the skipchain: " + err.Error())
	}
	priv := s.storage.Private[gid]
	if priv == nil {
		return nil, errors.New("don't have this identity stored")
	}

	// Verify darc
	// Note: The verify function needs the collection to be up to date.
	// TODO: Make sure that is the case.
	/*
			log.Lvl1("Verifying signature")
		    err := s.getCollection(req.SkipchainID).verify(&req.Transaction)
		    if err != nil {
				log.Lvl1("signature verification failed")
		        return nil, err
		    }
			log.Lvl1("signature verification succeeded")
	*/

	coll := s.getCollection(req.SkipchainID)
	if _, _, err := coll.GetValue(req.Transaction.Key); err == nil {
		return nil, errors.New("cannot overwrite existing value")
	}
	sigBuf, err := network.Marshal(&req.Transaction.Signature)
	if err != nil {
		return nil, errors.New("Couldn't marshal Signature: " + err.Error())
	}

	// Store the pair in a copy of the collection to get the root hash.
	// Once the block is accepted by the cothority, we store it in the real
	// collectionBD.
	var collCopy collection.Collection
	collCopy = s.getCollection(req.SkipchainID).coll
	collCopy.Add(req.Transaction.Key, req.Transaction.Value, sigBuf)
	mr := collCopy.GetRoot()
	data := &Data{
		MerkleRoot:   mr,
		Transactions: []*Transaction{&req.Transaction},
		Timestamp:    time.Now().Unix(),
	}

	buf, err := network.Marshal(data)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

	newBlock := latest.Copy()
	newBlock.Data = buf

	var ssb = skipchain.StoreSkipBlock{
		NewBlock:          newBlock,
		TargetSkipChainID: req.SkipchainID,
	} // TODO: Signature?
	ssbReply, err := s.skService().StoreSkipBlock(&ssb)
	if err != nil {
		return nil, err
	}

	// Now we know the block is accepted, so we can apply the the Transaction
	// to our collectionDB.
	err = coll.Store(req.Transaction.Key, req.Transaction.Value, sigBuf)
	if err != nil {
		return nil, errors.New(
			"error while storing in collection: " + err.Error())
	}

	hash := ssbReply.Latest.CalculateHash()
	return &SetKeyValueResponse{
		Version:     CurrentVersion,
		Timestamp:   &data.Timestamp,
		SkipblockID: &hash,
	}, nil
}

// GetProof searches for a key and returns a proof of the
// presence or the absence of this key.
func (s *Service) GetProof(req *GetProof) (resp *GetProofResponse, err error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}
	latest, err := s.db().GetLatest(s.db().GetByID(req.ID))
	if err != nil {
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

// saves all skipblocks.
func (s *Service) save() {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save([]byte(storageID), s.storage)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{}
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
	if s.storage.Private == nil {
		s.storage.Private = map[string]kyber.Scalar{}
	}
	s.collectionDB = map[string]*collectionDB{}

	gas := &skipchain.GetAllSkipchains{}
	gasr, err := s.skService().GetAllSkipchains(gas)
	if err != nil {
		return err
	}

	allSkipchains := gasr.SkipChains
	for _, sb := range allSkipchains {
		s.getCollection(sb.SkipChainID())
	}

	return nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real
// deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.CreateSkipchain, s.SetKeyValue,
		s.GetProof); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}

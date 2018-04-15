// Package service implements the lleap service using the collection library to
// handle the merkle-tree. Each call to SetKeyValue updates the Merkle-tree and
// creates a new block containing the root of the Merkle-tree plus the new value
// that has been stored last in the Merkle-tree.
package service

import (
	_ "crypto"
	_ "crypto/rsa"
	_ "crypto/sha256"
	_ "crypto/x509"
	"errors"
	"fmt"
	"sync"
    "time"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/identity"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/schnorr"
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
	network.RegisterMessage(&storage{})
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

type DarcBlock struct {
	sync.Mutex
	Latest          *Data
	LatestSkipblock *skipchain.SkipBlock
}

type Data struct {
    // Root of the merkle tree after applying the transactions to the
    // kv store
    MerkleRoot []byte
    // The transactions applied to the kv store with this block
    Transactions []*Transaction
    Timestamp int64
	Roster *onet.Roster
}

// storage is used to save our data locally.
type storage struct {
    // DarcBlock stores one skipchain together with the latest skipblock.
    DarcBlocks map[string]*DarcBlock
	// PL: Is used to sign the votes
	Private map[string]kyber.Scalar
	sync.Mutex
}

// CreateSkipchain asks the cisc-service to create a new skipchain ready to store
// key/value pairs. If it is given exactly one writer, this writer will be stored
// in the skipchain.
// For faster access, all data is also stored locally in the Service.storage
// structure.
func (s *Service) CreateSkipchain(req *CreateSkipchain) (*CreateSkipchainResponse, error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	kp := key.NewKeyPair(cothority.Suite)

    tmpColl := collection.New(collection.Data{}, collection.Data{})
    key := getKey(&req.Transaction)
    tmpColl.Add(key, req.Transaction.Value, req.Transaction.Signature)

    mr := tmpColl.GetRoot()
	data := &Data{
        MerkleRoot: mr,
        Transactions: []*Transaction{&req.Transaction},
        Timestamp: time.Now().Unix(),
	}

	buf, err := network.Marshal(data)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

    var genesisBlock = skipchain.NewSkipBlock()
	genesisBlock.Data = buf

    var ssb = skipchain.StoreSkipBlock{NewBlock: genesisBlock} // TODO: Signature?
    ssbReply, err := s.skService().StoreSkipBlock(&ssb)
	if err != nil {
		return nil, err
	}
    skID := ssbReply.Latest.SkipChainID()
	gid := string(skID)

	s.storage.DarcBlocks[gid] = &DarcBlock{
		Latest:          data,
		LatestSkipblock: ssbReply.Latest,
	}
    sigBuf, err := network.Marshal(req.Transaction.Signature)
	if err != nil {
		return nil, errors.New("Couldn't marshal Signature: " + err.Error())
	}
    err = s.getCollection(skID).Store(key, req.Transaction.Value, sigBuf)
	if err != nil {
		return nil, errors.New("error while storing in collection: " + err.Error())
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
	idb := s.storage.DarcBlocks[gid]
	priv := s.storage.Private[gid]
	if idb == nil || priv == nil {
		return nil, errors.New("don't have this identity stored")
	}

    // Verify darc
    // Note: The verify function needs the collection to be up to date.
    // TODO: Make sure that is the case.
	log.Lvl1("Verifying signature")
    err := s.getCollection(req.SkipchainID).verify(&req.Transaction)
    if err != nil {
		log.Lvl1("signature verification failed")
        return nil, err
    }
	log.Lvl1("signature verification succeeded")

	// Store the pair in the collection
	coll := s.getCollection(req.SkipchainID)
    key := getKey(&req.Transaction)
	if _, _, err := coll.GetValue(key); err == nil {
		return nil, errors.New("cannot overwrite existing value")
	}
    sigBuf, err := network.Marshal(req.Transaction.Signature)
	if err != nil {
		return nil, errors.New("Couldn't marshal Signature: " + err.Error())
	}
	err := coll.Store(key, req.Transaction.Value, sigBuf)
	if err != nil {
		return nil, errors.New("error while storing in collection: " + err.Error())
	}

	// Update the identity
	prop := idb.Latest.Copy()
	prop.Storage[keyMerkleRoot] = string(coll.RootHash())
	prop.Storage[keyNewKey] = string(req.Key)
	prop.Storage[keyNewValue] = string(req.Value)
	// TODO: Should also store the signature.
	// PL: What does this function do? What is the equivalent function to
	// interface directly with skipchain?
	_, err = s.idService().ProposeSend(&identity.ProposeSend{
		ID:      identity.ID(req.SkipchainID),
		Propose: prop,
	})
	if err != nil {
		return nil, err
	}

	hash, err := prop.Hash(cothority.Suite)
	if err != nil {
		return nil, err
	}
	sig, err := schnorr.Sign(cothority.Suite, priv, hash)
	if err != nil {
		return nil, err
	}
	// See the PL above
	resp, err := s.idService().ProposeVote(&identity.ProposeVote{
		ID:        identity.ID(req.SkipchainID),
		Signer:    "service",
		Signature: sig,
	})
	if err != nil {
		return nil, err
	}
	if resp.Data == nil {
		return nil, errors.New("couldn't store new skipblock")
	}
	timestamp := int64(resp.Data.Index)
	return &SetKeyValueResponse{
		Version:     CurrentVersion,
		Timestamp:   &timestamp,
		SkipblockID: &resp.Data.Hash,
	}, nil
}

// GetValue looks up the key in the given skipchain and returns the corresponding value.
func (s *Service) GetValue(req *GetValue) (*GetValueResponse, error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	value, sig, err := s.getCollection(req.SkipchainID).GetValue(req.Key)
	if err != nil {
		return nil, errors.New("couldn't get value for key: " + err.Error())
	}
	return &GetValueResponse{
		Version:   CurrentVersion,
		Value:     &value,
		Signature: &sig,
	}, nil
}

func getKey(tx *Transaction) []byte {
    return append(append(tx.Kind, []byte(":")...), tx.Key...)
}

func (s *Service) getCollection(id skipchain.SkipBlockID) *collectionDB {
	idStr := fmt.Sprintf("%x", id)
	col := s.collectionDB[idStr]
	if col == nil {
		db, name := s.GetAdditionalBucket([]byte(idStr))
		s.collectionDB[idStr] = newCollectionDB(db, string(name))
		return s.collectionDB[idStr]
	}
	return col
}

// interface to identity.Service
func (s *Service) idService() *identity.Service {
	return s.Service(identity.ServiceName).(*identity.Service)
}

func (s *Service) skService() *skipchain.Service {
    return s.Service(skipchain.ServiceName).(*skipchain.Service)
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
	if s.storage.DarcBlocks == nil {
		s.storage.DarcBlocks = map[string]*DarcBlock{}
	}
	if s.storage.Private == nil {
		s.storage.Private = map[string]kyber.Scalar{}
	}
	s.collectionDB = map[string]*collectionDB{}
	for _, ch := range s.storage.DarcBlocks {
		s.getCollection(ch.LatestSkipblock.SkipChainID())
	}
	return nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.CreateSkipchain, s.SetKeyValue,
		s.GetValue); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}

// Package service implements the lleap service using the collection library to
// handle the merkle-tree. Each call to SetKeyValue updates the Merkle-tree and
// creates a new block containing the root of the Merkle-tree plus the new value
// that has been stored last in the Merkle-tree.
package service

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/identity"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/schnorr"
	"gopkg.in/dedis/kyber.v2/util/key"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"
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
	Proposed        *Data
	LatestSkipblock *skipchain.SkipBlock
}

type Data struct {
    // Root of the merkle tree after applying the transactions to the
    // kv store
    MerkleRoot []byte
    // The transactions applied to the kv store with this block
    Transactions []*lleap.Transaction
    Timestamp int64
	Roster *onet.Roster
}

// storage is used to save our data locally.
type storage struct {
	// IDBlock stores one identity together with the latest and the currently
	// proposed skipblock.
	Identities map[string]*identity.IDBlock
	// PL: Is used to sign the votes
	Private map[string]kyber.Scalar
	// PL: Entities allowed to modify the data(-structure)?
	Writers map[string][]byte
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
	// get rid of identity here by using a struct which contains a DARC and
	// some sort of kv store.
	// Also adjust the structure of req *CreateSkipchain.
	// And what is writer? The device allowed to modify the kv-store (as well
	// the set of Devices)?
	data := &identity.Data{
		Threshold: 2,
		Device:    map[string]*identity.Device{"service": &identity.Device{Point: kp.Public}},
		Roster:    &req.Roster,
	}

	if req.Writers != nil && len(*req.Writers) == 1 {
		data.Storage = map[string]string{"writer": string((*req.Writers)[0])}
	}

	// replace this by something interacting with skipchain directly
	cir, err := s.idService().CreateIdentityInternal(&identity.CreateIdentity{
		Data: data,
	}, "", "")
	if err != nil {
		return nil, err
	}
	gid := string(cir.Genesis.SkipChainID())
	// if we modify data as described above, we can just use it here.
	// we can still use the genesisblock, but the one from skipchain
	s.storage.Identities[gid] = &identity.IDBlock{
		Latest:          data,
		LatestSkipblock: cir.Genesis,
	}
	s.storage.Private[gid] = kp.Private
	// s.storage.Writers[gid] = []byte(data.Storage["writer"])
	s.save()
	return &CreateSkipchainResponse{
		Version:   CurrentVersion,
		Skipblock: cir.Genesis,
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
	idb := s.storage.Identities[gid]
	priv := s.storage.Private[gid]
	if idb == nil || priv == nil {
		return nil, errors.New("don't have this identity stored")
	}
	// PL: Does this check make sense? What if pub == nil?
	if pub := s.storage.Writers[gid]; pub != nil {
		log.Lvl1("Verifying signature")
		public, err := x509.ParsePKIXPublicKey(pub)
		if err != nil || public == nil {
			return nil, err
		}
		hash := sha256.New()
		hash.Write(req.Key)
		hash.Write(req.Value)
		hashed := hash.Sum(nil)[:]
		err = rsa.VerifyPKCS1v15(public.(*rsa.PublicKey), crypto.SHA256, hashed, req.Signature)
		if err != nil {
			log.Lvl1("signature verification failed")
			return nil, errors.New("couldn't verify signature")
		}
		log.Lvl1("signature verification succeeded")
	}

	// Store the pair in the collection
	coll := s.getCollection(req.SkipchainID)
	if _, _, err := coll.GetValue(req.Key); err == nil {
		return nil, errors.New("cannot overwrite existing value")
	}
	err := coll.Store(req.Key, req.Value, req.Signature)
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
	if s.storage.Identities == nil {
		s.storage.Identities = map[string]*identity.IDBlock{}
	}
	if s.storage.Private == nil {
		s.storage.Private = map[string]kyber.Scalar{}
	}
	if s.storage.Writers == nil {
		s.storage.Writers = map[string][]byte{}
	}
	s.collectionDB = map[string]*collectionDB{}
	for _, id := range s.storage.Identities {
		s.getCollection(id.LatestSkipblock.SkipChainID())
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

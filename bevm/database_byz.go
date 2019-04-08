package bevm

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"sync"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3/log"

	"github.com/ethereum/go-ethereum/ethdb"
)

// ---------------------------------------------------------------------------
// Database distributed among Byzcoin value instances (base)

type ByzDatabase struct {
	bevmIID byzcoin.InstanceID // ID of the associated BEvmContract instance
}

func (db *ByzDatabase) getValueInstanceID(key []byte) byzcoin.InstanceID {
	// The instance ID of a value instance is given by the hash of the contract instance ID and the key

	h := sha256.New()
	h.Write(db.bevmIID[:])
	h.Write(key)

	return byzcoin.NewInstanceID(h.Sum(nil))
}

// ---------------------------------------------------------------------------
// ethdb.Database interface implementation (base)

// Close()
func (db *ByzDatabase) Close() {}

// ---------------------------------------------------------------------------
// Database distributed among Byzcoin value instances (client version)

type ClientByzDatabase struct {
	ByzDatabase
	client *byzcoin.Client
}

func NewClientByzDatabase(bevmIID byzcoin.InstanceID, client *byzcoin.Client) (*ClientByzDatabase, error) {
	return &ClientByzDatabase{
		ByzDatabase: ByzDatabase{
			bevmIID: bevmIID,
		},
		client: client,
	}, nil
}

// ---------------------------------------------------------------------------
// ethdb.Database interface implementation (client version)

// Putter
func (db *ClientByzDatabase) Put(key []byte, value []byte) error {
	return errors.New("Put() not allowed on ClientByzDatabase")
}

func (db *ClientByzDatabase) getBEvmValue(key []byte) ([]byte, error) {
	instID := db.getValueInstanceID(key)

	// Calling from a client

	if db.client == nil {
		return nil, errors.New("Internal error: both roStateTrie and client are nil")
	}

	// Retrieve the proof of the Byzcoin instance
	proofResponse, err := db.client.GetProof(instID[:])
	if err != nil {
		return nil, err
	}

	// Validate the proof
	err = proofResponse.Proof.Verify(db.client.ID)
	if err != nil {
		return nil, err
	}

	// Extract the value from the proof
	_, value, _, _, err := proofResponse.Proof.KeyValue()
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Has()
func (db *ClientByzDatabase) Has(key []byte) (bool, error) {
	_, err := db.getBEvmValue(key)

	return (err == nil), nil
}

// Get()
func (db *ClientByzDatabase) Get(key []byte) ([]byte, error) {
	value, err := db.getBEvmValue(key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Deleter
func (db *ClientByzDatabase) Delete(key []byte) error {
	return errors.New("Delete() not allowed on ClientByzDatabase")
}

// NewBatch()
func (db *ClientByzDatabase) NewBatch() ethdb.Batch {
	// NewBatch() not allowed on ClientByzDatabase
	return nil
}

// ---------------------------------------------------------------------------
// Database distributed among Byzcoin value instances (server version)

type ServerByzDatabase struct {
	ByzDatabase
	roStateTrie  byzcoin.ReadOnlyStateTrie
	stateChanges []byzcoin.StateChange // List of state changes to apply
	keys         map[string]bool       // Keeps track of existing value instances (identified by their key)
	lock         sync.RWMutex
}

func createKeyMap(keyList []string) map[string]bool {
	keys := make(map[string]bool)
	for _, key := range keyList {
		keys[key] = true
	}

	return keys
}

func NewServerByzDatabase(bevmIID byzcoin.InstanceID, keyList []string, roStateTrie byzcoin.ReadOnlyStateTrie) (*ServerByzDatabase, error) {
	return &ServerByzDatabase{
		ByzDatabase: ByzDatabase{
			bevmIID: bevmIID,
		},
		keys:        createKeyMap(keyList),
		roStateTrie: roStateTrie,
	}, nil
}

func (db *ServerByzDatabase) Dump() ([]byzcoin.StateChange, []string, error) {
	// The changes produced by the EVM are apparently not ordered deterministically.
	// Their order should, however, not be relevant, because each key is only affected by one change.
	// We can tehrefore sort them as we please, as long as the sort order is deterministic to make ByzCoin happy.

	// We check the hypothesis of unique keys before going further though...
	keyMap := make(map[string]bool)
	for _, s := range db.stateChanges {
		k := string(s.Key())
		if _, ok := keyMap[k]; ok {
			return nil, nil, errors.New("Internal error: the set of changes produced by the EVM is not unique on keys")
		}
	}

	// All good, let's sort by keys
	sort.SliceStable(db.stateChanges, func(i, j int) bool {
		return string(db.stateChanges[i].Key()) < string(db.stateChanges[j].Key())
	})

	var keyList []string
	for key, _ := range db.keys {
		keyList = append(keyList, key)
	}
	// This also must be sorted as Go maps traversal order is inherently non-deterministic
	sort.Strings(keyList)

	// Compute some statistics for information purposes
	nbCreate, nbUpdate, nbRemove := 0, 0, 0
	for _, s := range db.stateChanges {
		switch s.StateAction {
		case byzcoin.Create:
			nbCreate += 1
		case byzcoin.Update:
			nbUpdate += 1
		case byzcoin.Remove:
			nbRemove += 1
		default:
			return nil, nil, errors.New(fmt.Sprintf("Unknown StateChange action: %d", s.StateAction))
		}
	}
	log.LLvlf2("%d state changes (%d Create, %d Update, %d Remove), %d entries in store",
		len(db.stateChanges), nbCreate, nbUpdate, nbRemove, len(keyList))

	return db.stateChanges, keyList, nil
}

// ---------------------------------------------------------------------------
// ethdb.Database interface implementation (server version)

// Putter
func (db *ServerByzDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	return db.put(key, value)
}

// Actual implementation, callable from Batch.Write()
func (db *ServerByzDatabase) put(key []byte, value []byte) error {
	instanceID := db.getValueInstanceID(key)
	var sc byzcoin.StateChange

	if _, ok := db.keys[string(key)]; ok {
		sc = byzcoin.NewStateChange(byzcoin.Update, instanceID,
			ContractBEvmValueID, value, nil)
	} else {
		sc = byzcoin.NewStateChange(byzcoin.Create, instanceID,
			ContractBEvmValueID, value, nil)
	}
	db.keys[string(key)] = true

	db.stateChanges = append(db.stateChanges, sc)

	return nil
}

// Has()
func (db *ServerByzDatabase) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	_, ok := db.keys[string(key)]

	return ok, nil
}

// Get()
func (db *ServerByzDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	instID := db.getValueInstanceID(key)

	value, _, _, _, err := db.roStateTrie.GetValues(instID[:])
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Deleter
func (db *ServerByzDatabase) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	return db.delete(key)

	return nil
}

// Actual implementation, callable from Batch.Write()
func (db *ServerByzDatabase) delete(key []byte) error {
	instanceID := db.getValueInstanceID(key)

	sc := byzcoin.NewStateChange(byzcoin.Remove, instanceID,
		ContractBEvmValueID, nil, nil)

	db.stateChanges = append(db.stateChanges, sc)

	delete(db.keys, string(key))

	return nil
}

// NewBatch()
func (db *ServerByzDatabase) NewBatch() ethdb.Batch {
	return &MemBatch{db: db}
}

func (db *ServerByzDatabase) getLock() *sync.RWMutex {
	return &db.lock
}

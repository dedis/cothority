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

// ByzDatabase is the Ethereum state database distributed among ByzCoin value instances.
// It captures the common data between the client and server versions.
type ByzDatabase struct {
	bevmIID byzcoin.InstanceID // ID of the associated BEVM contract instance
}

// Compute the ByzCoin EVM value instance ID
func (db *ByzDatabase) getValueInstanceID(key []byte) byzcoin.InstanceID {
	// The instance ID of a value instance is given by the hash of the contract instance ID and the key

	h := sha256.New()
	h.Write(db.bevmIID[:])
	h.Write(key)

	return byzcoin.NewInstanceID(h.Sum(nil))
}

// ethdb.Database interface implementation (base)

// Close implements Close()
func (db *ByzDatabase) Close() {}

// ---------------------------------------------------------------------------

// ClientByzDatabase is the ByzDatabase version specialized for client
// (read-only) use, retrieving information using ByzCoin proofs
type ClientByzDatabase struct {
	ByzDatabase
	client *byzcoin.Client
}

// NewClientByzDatabase creates a new ByzDatabase for client use
func NewClientByzDatabase(bevmIID byzcoin.InstanceID, client *byzcoin.Client) (*ClientByzDatabase, error) {
	return &ClientByzDatabase{
		ByzDatabase: ByzDatabase{
			bevmIID: bevmIID,
		},
		client: client,
	}, nil
}

// ethdb.Database interface implementation (client version)

// Put implements Putter.Put()
func (db *ClientByzDatabase) Put(key []byte, value []byte) error {
	return errors.New("Put() not allowed on ClientByzDatabase")
}

// Retrieve the value from a BEVM value instance
func (db *ClientByzDatabase) getBEvmValue(key []byte) ([]byte, error) {
	instID := db.getValueInstanceID(key)

	// Retrieve the proof of the BEvmValue instance
	proofResponse, err := db.client.GetProof(instID[:])
	if err != nil {
		return nil, errors.New("error retrieving BEvmValue instance: " + err.Error())
	}

	// Validate the proof
	err = proofResponse.Proof.Verify(db.client.ID)
	if err != nil {
		return nil, errors.New("error verifying BEvmValue instance proof: " + err.Error())
	}

	// Extract the value from the proof
	_, value, _, _, err := proofResponse.Proof.KeyValue()
	if err != nil {
		return nil, errors.New("error getting BEvmValue instance value: " + err.Error())
	}

	return value, nil
}

// Has implements Has()
func (db *ClientByzDatabase) Has(key []byte) (bool, error) {
	_, err := db.getBEvmValue(key)

	return (err == nil), nil
}

// Get implements Get()
func (db *ClientByzDatabase) Get(key []byte) ([]byte, error) {
	value, err := db.getBEvmValue(key)
	if err != nil {
		return nil, fmt.Errorf("error getting value for key '%v': %s", key, err.Error())
	}

	return value, nil
}

// Delete implements Deleter.Delete()
func (db *ClientByzDatabase) Delete(key []byte) error {
	return errors.New("Delete() not allowed on ClientByzDatabase")
}

// NewBatch implements NewBatch()
func (db *ClientByzDatabase) NewBatch() ethdb.Batch {
	// NewBatch() not allowed on ClientByzDatabase
	return nil
}

// ---------------------------------------------------------------------------

// ServerByzDatabase is the ByzDatabase version specialized for server
// (read/write) use, updating ByzCoin via StateChanges
type ServerByzDatabase struct {
	ByzDatabase
	roStateTrie  byzcoin.ReadOnlyStateTrie
	stateChanges []byzcoin.StateChange // List of state changes to apply
	keyMap       map[string]bool       // Keeps track of existing value instances (identified by their key)
	lock         sync.RWMutex          // Protects concurrent access to 'keyMap' and 'stateChanges'
}

func keyListToMap(keyList []string) map[string]bool {
	keyMap := make(map[string]bool)
	for _, key := range keyList {
		keyMap[key] = true
	}

	return keyMap
}

func keyMapToList(keyMap map[string]bool) []string {
	var keyList []string

	for key := range keyMap {
		keyList = append(keyList, key)
	}
	// The list must be sorted as Go maps traversal order is inherently non-deterministic
	sort.Strings(keyList)

	return keyList
}

// NewServerByzDatabase creates a new ByzDatabase for server use
func NewServerByzDatabase(bevmIID byzcoin.InstanceID, keyList []string, roStateTrie byzcoin.ReadOnlyStateTrie) (*ServerByzDatabase, error) {
	return &ServerByzDatabase{
		ByzDatabase: ByzDatabase{
			bevmIID: bevmIID,
		},
		keyMap:      keyListToMap(keyList),
		roStateTrie: roStateTrie,
	}, nil
}

// Dump returns the list of StateChanges to apply to ByzCoin as well as the
// list of keys in the Ethereum state database, representing the modifications
// that the EVM performed on its state database
func (db *ServerByzDatabase) Dump() ([]byzcoin.StateChange, []string, error) {
	// The changes produced by the EVM are apparently not ordered
	// deterministically. Their order should, however, not be relevant,
	// because each key is only affected by one change. We can tehrefore sort
	// them as we please, as long as the sort order is deterministic to make
	// ByzCoin happy.

	// We check the hypothesis of unique keys before going further though...
	keyMap := make(map[string]string)
	for _, s := range db.stateChanges {
		k := string(s.Key())
		if val, ok := keyMap[k]; ok && val != string(s.Value) {
			return nil, nil, errors.New("internal error: the set of changes produced by the EVM is not unique on keys")
		}
		keyMap[k] = string(s.Value)
	}

	// All good, let's sort by keys
	sort.SliceStable(db.stateChanges, func(i, j int) bool {
		return string(db.stateChanges[i].Key()) < string(db.stateChanges[j].Key())
	})

	keyList := keyMapToList(db.keyMap)

	// Compute some statistics for information purposes
	nbCreate, nbUpdate, nbRemove := 0, 0, 0
	for _, s := range db.stateChanges {
		switch s.StateAction {
		case byzcoin.Create:
			nbCreate++
		case byzcoin.Update:
			nbUpdate++
		case byzcoin.Remove:
			nbRemove++
		default:
			return nil, nil, fmt.Errorf("unknown StateChange action: %d", s.StateAction)
		}
	}
	log.Lvlf2("%d state changes (%d Create, %d Update, %d Remove), %d entries in store",
		len(db.stateChanges), nbCreate, nbUpdate, nbRemove, len(keyList))

	return db.stateChanges, keyList, nil
}

// ethdb.Database interface implementation (server version)

// Put implements Putter.Put()
func (db *ServerByzDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	return db.put(key, value)
}

// Implements lowLevelDb.put()
func (db *ServerByzDatabase) put(key []byte, value []byte) error {
	instanceID := db.getValueInstanceID(key)
	var sc byzcoin.StateChange

	if _, ok := db.keyMap[string(key)]; ok {
		sc = byzcoin.NewStateChange(byzcoin.Update, instanceID,
			ContractBEvmValueID, value, nil)
	} else {
		sc = byzcoin.NewStateChange(byzcoin.Create, instanceID,
			ContractBEvmValueID, value, nil)
	}
	db.keyMap[string(key)] = true

	db.stateChanges = append(db.stateChanges, sc)

	return nil
}

// Has implements Has()
func (db *ServerByzDatabase) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	_, ok := db.keyMap[string(key)]

	return ok, nil
}

// Get implements Get()
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

// Delete implements Deleter.Delete()
func (db *ServerByzDatabase) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	return db.delete(key)
}

// Implements lowLevelDb.delete()
func (db *ServerByzDatabase) delete(key []byte) error {
	instanceID := db.getValueInstanceID(key)

	sc := byzcoin.NewStateChange(byzcoin.Remove, instanceID,
		ContractBEvmValueID, nil, nil)

	db.stateChanges = append(db.stateChanges, sc)

	delete(db.keyMap, string(key))

	return nil
}

// NewBatch implements NewBatch()
func (db *ServerByzDatabase) NewBatch() ethdb.Batch {
	return &memBatch{db: db}
}

func (db *ServerByzDatabase) getLock() *sync.RWMutex {
	return &db.lock
}

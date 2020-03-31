package byzcoin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	//bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/byzcoin/collection"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	bolt "github.com/coreos/bbolt"
)

func init() {
	network.RegisterMessages(&darc.Signature{},
		DataHeader{}, DataBody{})
}

type collectionDB struct {
	db         *bolt.DB
	bucketName []byte
	coll       *collection.Collection
	scID       skipchain.SkipBlockID
}

// A CollectionView is an interface that defines the read-only operations
// on a collection.
type CollectionView interface {
	// Get returns the collection.Getter for the given key.
	// collection.Getter is valid even for a non-existing key.
	Get(key []byte) collection.Getter
	// GetValues returns the value, contractID, and owning darc ID for the given key, or
	// an error if something went wrong. A non-existing key returns an
	// error.
	GetValues(key []byte) (value []byte, contractID string, darcID darc.ID, err error)
}

// roCollection is a wrapper for a collection that satisfies interface
// CollectionView and makes it impossible for callers who receive it to call
// the methods on the collection which can modify it. This is about type
// safety, not real security. If the holder of the CollectionView chooses to
// use package unsafe, then it's all over; they can get write access.
type roCollection struct {
	c *collection.Collection
}

// Get returns the collection.Getter for the key.
func (r *roCollection) Get(key []byte) collection.Getter {
	return r.c.Get(key)
}

// GetValues returns the value of the key and the contractID. If the key
// does not exist, it returns an error.
func (r *roCollection) GetValues(key []byte) (value []byte, contractID string, darcID darc.ID, err error) {
	return getValueContract(r, key)
}

// ContractFn is the type signature of the class functions
// which can be registered with the ByzCoin service.
type ContractFn func(coll CollectionView, inst Instruction, inCoins []Coin) (sc []StateChange, outCoins []Coin, err error)

// newCollectionDB initialises a structure and reads all key/value pairs to store
// it in the collection.
func newCollectionDB(db *bolt.DB, name []byte) *collectionDB {
	c := &collectionDB{
		db:         db,
		bucketName: name,
		coll:       collection.New(collection.Data{}, collection.Data{}, collection.Data{}),
	}
	c.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(name)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	err := c.loadAll()
	if err != nil {
		log.Error("unable to load collection from disk:", err)
	}

	// TODO: Check the merkle tree root.
	return c
}

// dup makes a copy of in. We use this with results from BoltDB
// because BoltDB's docs say, "The returned value is only valid for
// the life of the transaction."
func dup(in []byte) []byte {
	return append([]byte{}, in...)
}

const (
	dbValue byte = iota
	dbContract
	dbDarcID
	dbMeta
)

const (
	dbMetaIndex byte = iota
)

func (c *collectionDB) loadAll() error {
	return c.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(c.bucketName))
		cur := b.Cursor()

		for k, v := cur.First(); k != nil; k, v = cur.Next() {
			// Only look at value keys
			if len(k) > 0 && k[0] != dbValue {
				continue
			}

			k2 := dup(k)
			k2[0] = dbContract
			cv := b.Get(k2)
			if cv == nil {
				return fmt.Errorf("contract type missing for object ID %x", k[1:])
			}

			k2[0] = dbDarcID
			dv := b.Get(k2)
			if dv == nil {
				return fmt.Errorf("darcID missing for object ID %x", k[1:])
			}

			err := c.coll.Add(dup(k[1:]), dup(v), dup(cv), dup(dv))
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func storeInColl(coll *collection.Collection, t *StateChange) error {
	switch t.StateAction {
	case Create:
		return coll.Add(t.InstanceID, t.Value, t.ContractID, []byte(t.DarcID))
	case Update:
		return coll.Set(t.InstanceID, t.Value, t.ContractID, []byte(t.DarcID))
	case Remove:
		return coll.Remove(t.InstanceID)
	default:
		return errors.New("invalid state action")
	}
}

func (c *collectionDB) Get(key []byte) collection.Getter {
	return c.coll.Get(key)
}

func (c *collectionDB) GetValues(key []byte) (value []byte, contractID string, darcID darc.ID, err error) {
	return getValueContract(c, key)
}

// Look up the index number of the skipblock that held the most recently
// applied state changes. On error, it returns index -1, which callers
// might want (new chain case) or might detect as an error (existing
// chain, block arriving has index >=1, so getIndex() == -1 -> do not
// accept.
func (c *collectionDB) getIndex() int {
	var out uint32
	err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucketName))
		if bucket == nil {
			return errors.New("bucket does not exist")
		}

		b := bucket.Get([]byte{dbMeta, dbMetaIndex})
		if len(b) == 4 {
			out = binary.LittleEndian.Uint32(b)
		} else {
			return errors.New("collection index not found")
		}
		return nil
	})
	if err != nil {
		return -1
	}
	return int(out)
}

// FIXME: if there is an error, the data in collection may not be consistent
// with boltdb.
func (c *collectionDB) StoreAll(ts StateChanges, index int) error {
	for _, t := range ts {
		if err := storeInColl(c.coll, &t); err != nil {
			return err
		}
	}
	return c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucketName))
		if bucket == nil {
			return errors.New("bucket does not exist")
		}

		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(index))
		if err := bucket.Put([]byte{dbMeta, dbMetaIndex}, b); err != nil {
			return err
		}

		for _, t := range ts {
			key := make([]byte, 1+len(t.InstanceID))
			copy(key[1:], t.InstanceID)

			switch t.StateAction {
			case Create, Update:
				key[0] = dbValue
				if err := bucket.Put(key, t.Value); err != nil {
					return err
				}
				key[0] = dbContract
				if err := bucket.Put(key, t.ContractID); err != nil {
					return err
				}
				key[0] = dbDarcID
				if err := bucket.Put(key, t.DarcID); err != nil {
					return err
				}
			case Remove:
				key[0] = dbValue
				if err := bucket.Delete(key); err != nil {
					return err
				}
				key[0] = dbContract
				if err := bucket.Delete(key); err != nil {
					return err
				}
				key[0] = dbDarcID
				if err := bucket.Delete(key); err != nil {
					return err
				}
			default:
				return errors.New("invalid state action")
			}
		}
		return nil
	})
}

// RootHash returns the hash of the root node in the merkle tree.
func (c *collectionDB) RootHash() []byte {
	return c.coll.GetRoot()
}

var errKeyNotSet = errors.New("key not set")

func getValueContract(coll CollectionView, key []byte) (value []byte, contract string, darcID darc.ID, err error) {
	record, err := coll.Get(key).Record()
	if err != nil {
		return
	}
	if !record.Match() {
		err = errKeyNotSet
		return
	}
	values, err := record.Values()
	if err != nil {
		return
	}

	if len(values) == 0 {
		err = errors.New("nothing stored under that key")
		return
	}
	if len(values) != 3 {
		err = errors.New("wrong number of values")
		return
	}

	value, ok := values[0].([]byte)
	if !ok {
		err = errors.New("the value is not of type []byte")
		return
	}
	contractBytes, ok := values[1].([]byte)
	if !ok {
		err = errors.New("the contract is not of type []byte")
		return
	}
	contract = string(contractBytes)

	darcIDBytes, ok := values[2].([]byte)
	if !ok {
		err = errors.New("the darcID is not of type []byte")
		return
	}
	darcID = darc.ID(darcIDBytes)

	return
}

// tryHash returns the merkle root of the collection as if the key value pairs
// in the transactions had been added, without actually adding it.
func (c *collectionDB) tryHash(ts []StateChange) (mr []byte, rerr error) {
	for _, sc := range ts {
		err := c.coll.Add(sc.InstanceID, sc.Value, sc.ContractID, []byte(sc.DarcID))
		if err != nil {
			rerr = err
			return
		}
		// remove the pair after we got the merkle root.
		defer func(k []byte) {
			err = c.coll.Remove(k)
			if err != nil {
				rerr = err
				mr = nil
			}
		}(sc.InstanceID)
	}
	mr = c.coll.GetRoot()
	return
}

func getInstanceDarc(c CollectionView, iid InstanceID) (*darc.Darc, error) {
	// From instance ID, find the darcID that controls access to it.
	_, _, dID, err := c.GetValues(iid.Slice())
	if err != nil {
		return nil, err
	}

	// Fetch the darc itself.
	value, contract, _, err := c.GetValues(dID)
	if err != nil {
		return nil, err
	}

	if string(contract) != ContractDarcID {
		return nil, fmt.Errorf("for instance %v, expected Kind to be 'darc' but got '%v'", iid, string(contract))
	}
	return darc.NewFromProtobuf(value)
}

// RegisterContract stores the contract in a map and will
// call it whenever a contract needs to be done.
// GetService makes it possible to give either an `onet.Context` or
// `onet.Server` to `RegisterContract`.
func RegisterContract(s skipchain.GetService, kind string, f ContractFn) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerContract(kind, f)
}

// SafeAdd will add a to the value of the coin if there will be no
// overflow.
func (c *Coin) SafeAdd(a uint64) error {
	s1 := c.Value + a
	if s1 < c.Value || s1 < a {
		return errors.New("uint64 overflow")
	}
	c.Value = s1
	return nil
}

// SafeSub subtracts a from the value of the coin if there
// will be no underflow.
func (c *Coin) SafeSub(a uint64) error {
	if a <= c.Value {
		c.Value -= a
		return nil
	}
	return errors.New("uint64 underflow")
}

type bcState struct {
	sync.Mutex
	// waitChannels will be informed by Service.updateCollection that a
	// given ClientTransaction has been included. updateCollection will
	// send true for a valid ClientTransaction and false for an invalid
	// ClientTransaction.
	waitChannels map[string]chan bool
	// blockListeners will be notified every time a block is created.
	// It is up to them to filter out block creations on chains they are not
	// interested in.
	blockListeners []chan skipchain.SkipBlockID
}

func (bc *bcState) createWaitChannel(ctxHash []byte) chan bool {
	bc.Lock()
	defer bc.Unlock()
	ch := make(chan bool, 1)
	bc.waitChannels[string(ctxHash)] = ch
	return ch
}

func (bc *bcState) informWaitChannel(ctxHash []byte, valid bool) {
	bc.Lock()
	defer bc.Unlock()
	ch := bc.waitChannels[string(ctxHash)]
	if ch != nil {
		ch <- valid
	}
}

func (bc *bcState) deleteWaitChannel(ctxHash []byte) {
	bc.Lock()
	defer bc.Unlock()
	delete(bc.waitChannels, string(ctxHash))
}

func (bc *bcState) informBlock(id skipchain.SkipBlockID) {
	bc.Lock()
	defer bc.Unlock()
	for _, x := range bc.blockListeners {
		if x != nil {
			x <- id
		}
	}
}

func (bc *bcState) registerForBlocks(ch chan skipchain.SkipBlockID) int {
	bc.Lock()
	defer bc.Unlock()

	for i := 0; i < len(bc.blockListeners); i++ {
		if bc.blockListeners[i] == nil {
			bc.blockListeners[i] = ch
			return i
		}
	}

	// If we got here, no empty spots left, append and return the position of the
	// new element (on startup: after append(nil, ch), len == 1, so len-1 = index 0.
	bc.blockListeners = append(bc.blockListeners, ch)
	return len(bc.blockListeners) - 1
}

func (bc *bcState) unregisterForBlocks(i int) {
	bc.Lock()
	defer bc.Unlock()
	bc.blockListeners[i] = nil
}

func (c ChainConfig) sanityCheck() error {
	if c.BlockInterval <= 0 {
		return errors.New("block interval is less or equal to zero")
	}
	// too small would make it impossible to even send through a config update tx to fix it,
	// so don't allow that.
	if c.MaxBlockSize < 16000 {
		return errors.New("max block size is less than 16000")
	}
	// onet/network.MaxPacketSize is 10 megs, leave some headroom anyway.
	if c.MaxBlockSize > 8*1e6 {
		return errors.New("max block size is greater than 8 megs")
	}
	return nil
}

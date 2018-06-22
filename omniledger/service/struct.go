package service

import (
	"errors"
	"fmt"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/omniledger/collection"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet/network"
)

func init() {
	network.RegisterMessages(&darc.Signature{},
		DataHeader{}, DataBody{})
}

type collectionDB struct {
	db         *bolt.DB
	bucketName []byte
	coll       collection.Collection
}

// A CollectionView is an interface that defines the read-only operations
// on a collection.
type CollectionView interface {
	Get(key []byte) collection.Getter
	GetValues(key []byte) (value []byte, contractID string, err error)
	GetValue(key []byte) ([]byte, error)
	GetContractID(key []byte) (string, error)
}

// roCollection is a wrapper for a collection that satisfies interface
// CollectionView and makes it impossible for callers who receive it
// to call the methods on the collection which can modify it. This
// is about type safety, not real security. If the holder of the
// CollectionView chooses to use package unsafe, then it's all over;
// they can get write access.
type roCollection struct {
	c collection.Collection
}

func (r *roCollection) Get(key []byte) collection.Getter {
	return r.c.Get(key)
}

func (r *roCollection) GetValues(key []byte) (value []byte, contractID string, err error) {
	record, err := r.c.Get(key).Record()
	if err != nil {
		return
	}
	values, err := record.Values()
	if err != nil {
		return
	}
	var ok bool
	value, ok = values[0].([]byte)
	if !ok {
		err = errors.New("first value is not a slice of bytes")
		return
	}
	contractID, ok = values[1].(string)
	if !ok {
		err = errors.New("second value is not a string")
	}
	return
}

func (r *roCollection) GetValue(key []byte) ([]byte, error) {
	v, _, err := r.GetValues(key)
	return v, err
}
func (r *roCollection) GetContractID(key []byte) (string, error) {
	_, c, err := r.GetValues(key)
	return c, err
}

// OmniLedgerContract is the type signature of the class functions
// which can be registered with the omniledger service.
// Since the outcome of the verification depends on the state of the collection
// which is to be modified, we pass it as a pointer here.
type OmniLedgerContract func(coll CollectionView, tx Instruction, inCoins []Coin) (sc []StateChange, outCoins []Coin, err error)

// newCollectionDB initialises a structure and reads all key/value pairs to store
// it in the collection.
func newCollectionDB(db *bolt.DB, name []byte) *collectionDB {
	c := &collectionDB{
		db:         db,
		bucketName: name,
		coll:       collection.New(collection.Data{}, collection.Data{}),
	}
	c.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(name)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	c.loadAll()
	// TODO: Check the merkle tree root.
	return c
}

// dup makes a copy of in. We use this with results from BoltDB
// because BoltDB's docs say, "The returned value is only valid for
// the life of the transaction."
func dup(in []byte) []byte {
	return append([]byte{}, in...)
}

func (c *collectionDB) loadAll() error {
	return c.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(c.bucketName))
		cur := b.Cursor()

		for k, v := cur.First(); k != nil; k, v = cur.Next() {
			// This is a Contract key, skip it.
			if len(k) > 0 && k[0] == 'C' {
				continue
			}
			kc := make([]byte, len(k)+1)
			kc[0] = 'C'
			copy(kc[1:], k)

			cv := b.Get(kc)
			if cv == nil {
				return fmt.Errorf("contract ype missing for object ID %x", k)
			}
			err := c.coll.Add(dup(k), dup(v), dup(cv))
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func storeInColl(coll collection.Collection, t *StateChange) error {
	switch t.StateAction {
	case Create:
		return coll.Add(t.InstanceID, t.Value, t.ContractID)
	case Update:
		return coll.Set(t.InstanceID, t.Value, t.ContractID)
	case Remove:
		return coll.Remove(t.InstanceID)
	default:
		return errors.New("invalid state action")
	}
}

func (c *collectionDB) Store(t *StateChange) error {
	if err := storeInColl(c.coll, t); err != nil {
		return err
	}
	err := c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucketName))

		// The contract type is stored in a key starting with C
		keyC := make([]byte, 1+len(t.InstanceID))
		keyC[0] = byte('C')
		copy(keyC[1:], t.InstanceID)

		switch t.StateAction {
		case Create, Update:
			if err := bucket.Put(t.InstanceID, t.Value); err != nil {
				return err
			}
			return bucket.Put(keyC, t.ContractID)
		case Remove:
			if err := bucket.Delete(t.InstanceID); err != nil {
				return err
			}
			return bucket.Delete(keyC)
		default:
			return errors.New("invalid state action")
		}
	})
	return err
}

func (c *collectionDB) GetValueContract(key []byte) ([]byte, []byte, error) {
	return getValueContract(&roCollection{c.coll}, key)
}

func getValueContract(coll CollectionView, key []byte) (value, contract []byte, err error) {
	proof, err := coll.Get(key).Record()
	if err != nil {
		return
	}
	hashes, err := proof.Values()
	if err != nil {
		return
	}
	if len(hashes) == 0 {
		err = errors.New("nothing stored under that key")
		return
	}
	value, ok := hashes[0].([]byte)
	if !ok {
		err = errors.New("the value is not of type []byte")
		return
	}
	contract, ok = hashes[1].([]byte)
	if !ok {
		err = errors.New("the contract is not of type []byte")
		return
	}
	return
}

// RootHash returns the hash of the root node in the merkle tree.
func (c *collectionDB) RootHash() []byte {
	return c.coll.GetRoot()
}

// tryHash returns the merkle root of the collection as if the key value pairs
// in the transactions had been added, without actually adding it.
func (c *collectionDB) tryHash(ts []StateChange) (mr []byte, rerr error) {
	for _, sc := range ts {
		err := c.coll.Add(sc.InstanceID, sc.Value, sc.ContractID)
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

// RegisterContract stores the contract in a map and will
// call it whenever a contract needs to be done.
// GetService makes it possible to give either an `onet.Context` or
// `onet.Server` to `RegisterContract`.
func RegisterContract(s skipchain.GetService, kind string, f OmniLedgerContract) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerContract(kind, f)
}

// DataHeader is the data passed to the Skipchain
type DataHeader struct {
	// CollectionRoot is the root of the merkle tree of the colleciton after
	// applying the valid transactions.
	CollectionRoot []byte
	// ClientTransactionHash is the sha256 hash of all the transactions in the body
	ClientTransactionHash []byte
	// StateChangesHash is the sha256 of all the stateChanges occuring through the
	// clientTransactions.
	StateChangesHash []byte
	// Timestamp is a unix timestamp in nanoseconds.
	Timestamp int64
}

// DataBody is stored in the body of the skipblock but is not hashed. This reduces
// the proof needed for a key/value pair.
type DataBody struct {
	Transactions ClientTransactions
}

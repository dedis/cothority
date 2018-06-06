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

// OmniLedgerContract is the type signature of the class functions
// which can be registered with the omniledger service.
// Since the outcome of the verification depends on the state of the collection
// which is to be modified, we pass it as a pointer here.
type OmniLedgerContract func(cdb collection.Collection, tx Instruction, inCoins []Coin) (sc []StateChange, outCoins []Coin, err error)

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
		return coll.Add(t.ObjectID, t.Value, t.ContractID)
	case Update:
		return coll.Set(t.ObjectID, t.Value, t.ContractID)
	case Remove:
		return coll.Remove(t.ObjectID)
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
		keyC := make([]byte, 1+len(t.ObjectID))
		keyC[0] = byte('C')
		copy(keyC[1:], t.ObjectID)

		switch t.StateAction {
		case Create, Update:
			if err := bucket.Put(t.ObjectID, t.Value); err != nil {
				return err
			}
			return bucket.Put(keyC, t.ContractID)
		case Remove:
			if err := bucket.Delete(t.ObjectID); err != nil {
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
	return getValueContract(c.coll, key)
}

func getValueContract(coll collection.Collection, key []byte) (value, contract []byte, err error) {
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
		err := c.coll.Add(sc.ObjectID, sc.Value, sc.ContractID)
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
		}(sc.ObjectID)
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

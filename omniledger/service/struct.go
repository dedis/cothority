package service

import (
	"errors"
	"fmt"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/onet"
	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/onet.v2/network"
)

func init() {
	network.RegisterMessages(&Transaction{},
		&darc.Signature{})
}

type collectionDB struct {
	db         *bolt.DB
	bucketName []byte
	coll       collection.Collection
}

type OmniledgerVerifier func(cdb *collectionDB, tx *Transaction) bool

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

func (c *collectionDB) loadAll() {
	c.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(c.bucketName))
		cur := b.Cursor()

		for k, v := cur.First(); k != nil; k, v = cur.Next() {
			sig := b.Get(append(k, []byte("sig")...))
			c.coll.Add(k, v, sig)
		}

		return nil
	})
}

func (c *collectionDB) Store(t *Transaction) error {
	c.coll.Add(t.Key, t.Value, t.Kind)
	err := c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucketName))
		if err := bucket.Put(t.Key, t.Value); err != nil {
			return err
		}
		keykind := make([]byte, len(t.Key)+4)
		copy(keykind, t.Key)
		keykind = append(keykind, []byte("kind")...)
		if err := bucket.Put(keykind, t.Kind); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (c *collectionDB) GetValueKind(key []byte) (value, kind []byte, err error) {
	proof, err := c.coll.Get(key).Record()
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
	kind, ok = hashes[1].([]byte)
	if !ok {
		err = errors.New("the signature is not of type []byte")
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
func (c *collectionDB) tryHash(ts []Transaction) (mr []byte, rerr error) {
	for _, t := range ts {
		err := c.coll.Add(t.Key, t.Value, t.Kind)
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
		}(t.Key)
	}
	mr = c.coll.GetRoot()
	return
}

// Action describes how the collectionDB will be modified.
type Action int

const (
	// Create allows to insert a new key-value association.
	Create Action = iota + 1
	// Update allows to change the value of an existing key.
	Update
	// Remove allows to delete an existing key-value association.
	Remove
)

// Transaction is the struct specifying the modifications to the skipchain.
// Key is the key chosen by the user, Kind is the kind of value to store
// (e.g. a drac...). The key used in the conode's collection will be
// Kind ':' Key, in order to maintain key uniqueness across different kinds
// of values.
// For a Transaction to be valid, there must exist a path from the master-darc
// in the genesis block to the SubjectPK in Signature.
type Transaction struct {
	Action Action
	Key    []byte
	Kind   []byte
	Value  []byte
	// The signature is performed on the concatenation of the []bytes
	Signature darc.Signature
}

// Data is the data passed to the Skipchain
type Data struct {
	// Root of the merkle tree after applying the transactions to the
	// kv store
	MerkleRoot []byte
	// The transactions applied to the kv store with this block
	Transactions []Transaction
	Timestamp    int64
	Roster       *onet.Roster
}

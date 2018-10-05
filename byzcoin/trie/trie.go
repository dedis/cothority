package trie

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	bolt "github.com/coreos/bbolt"
)

// this is where the root hash is stored
const entryKey = "dedis_trie"
const nonceKey = "dedis_trie_nonce"

// Trie implements the merkle prefix tree described in the coniks paper.
type Trie struct {
	nonce  []byte
	db     *bolt.DB
	bucket []byte
}

// NewTrie loads the tried from a boltDB database, it creates one if it does
// not exist.
func NewTrie(db *bolt.DB, bucket []byte) (Trie, error) {
	nonce := make([]byte, 32)
	err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return err
		}

		// create or load the nonce
		nonceBuf := b.Get([]byte(nonceKey))
		if nonceBuf == nil {
			nonce = genNonce()
			if err := b.Put([]byte(nonceKey), nonce); err != nil {
				return err
			}
		} else {
			copy(nonce, nonceBuf)
		}

		// create or load the root node
		rootKey := b.Get([]byte(entryKey))
		if rootKey == nil {
			err := newRootNode(b, nonce)
			if err != nil {
				return err
			}
			return nil
		}
		rootVal := b.Get([]byte(rootKey))
		if rootVal == nil {
			return errors.New("invalid reference to root")
		}
		return err
	})

	if err != nil {
		return Trie{}, err
	}
	return Trie{
		nonce:  nonce,
		db:     db,
		bucket: bucket,
	}, nil
}

// newRootNode creates the root node and two empty nodes and store these in the
// bucket.
func newRootNode(b *bolt.Bucket, nonce []byte) error {
	left := newEmptyNode([]bool{true})
	right := newEmptyNode([]bool{false})
	root := newInteriorNode(left.hash(nonce), right.hash(nonce))

	// encode the buffers
	leftBuf, err := left.encode()
	if err != nil {
		return err
	}
	rightBuf, err := right.encode()
	if err != nil {
		return err
	}
	rootBuf, err := root.encode()
	if err != nil {
		return err
	}

	// put them into the database
	if err := b.Put(left.hash(nonce), leftBuf); err != nil {
		return err
	}
	if err := b.Put(right.hash(nonce), rightBuf); err != nil {
		return err
	}
	if err := b.Put(root.hash(), rootBuf); err != nil {
		return err
	}

	// update the entry key
	if err := b.Put([]byte(entryKey), root.hash()); err != nil {
		return err
	}
	return nil
}

// Set sets or overwrites a key-value pair.
func (t *Trie) Set(key []byte, value []byte) error {
	return t.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(t.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		newRoot, err := t.set(t.getRoot(b), toBinarySlice(key), 0, key, value, b)
		if err != nil {
			return err
		}
		return b.Put([]byte(entryKey), newRoot)
	})
}

func (t *Trie) getRoot(b *bolt.Bucket) []byte {
	return b.Get([]byte(entryKey))
}

func (t *Trie) set(nodeKey []byte, bits []bool, depth int, key, value []byte, b *bolt.Bucket) ([]byte, error) {
	nodeVal := b.Get(nodeKey)
	if len(nodeVal) == 0 {
		return nil, errors.New("invalid node key")
	}
	switch nodeType(nodeVal[0]) {
	case typeEmpty:
		// base case 1
		println("EMPTY")
		node, err := decodeEmptyNode(nodeVal)
		if err != nil {
			return nil, err
		}
		return t.emptyToLeaf(node, key, value, b)
	case typeLeaf:
		// base case 2
		println("LEAF")
		node, err := decodeLeafNode(nodeVal)
		if err != nil {
			return nil, err
		}

		// If the key is the same, then we don't need to create a new
		// internal node, just update the value and hash.
		if bytes.Equal(node.Key, key) {
			oldValueKey := node.DataKey
			valueKey := sha256.Sum256(value)
			node.DataKey = valueKey[:]
			if err := b.Delete(oldValueKey); err != nil {
				return nil, err
			}
			leafBuf, err := node.encode()
			if err != nil {
				return nil, err
			}
			if err := b.Put(node.hash(t.nonce), leafBuf); err != nil {
				return nil, err
			}
			if err := b.Put(valueKey[:], value); err != nil {
				return nil, err
			}
			return node.hash(t.nonce), nil
		}
		// Otherwise, we need to create one or more interior nodes.
		valueKey := sha256.Sum256(value)
		left, right, err := t.extendLeaf(node.Prefix, node.Key, node.DataKey, key, valueKey[:], b)
		if err != nil {
			return nil, err
		}
		// Store the new interior node.
		interior := newInteriorNode(left, right)
		interiorBuff, err := interior.encode()
		if err != nil {
			return nil, err
		}
		if err := b.Put(interior.hash(), interiorBuff); err != nil {
			return nil, err
		}
		// Store the new value.
		if err := b.Put(valueKey[:], value); err != nil {
			return nil, err
		}
		// Delete the old leaf node.
		if err := b.Delete(node.hash(t.nonce)); err != nil {
			return nil, err
		}
		return interior.hash(), nil
	case typeInterior:
		// recursive case
		print("INTERIOR")
		node, err := decodeInteriorNode(nodeVal)
		if err != nil {
			return nil, err
		}
		var retHash []byte
		if bits[depth] {
			println(" LEFT")
			retHash, err = t.set(node.Left, bits, depth+1, key, value, b)
			if err != nil {
				return nil, err
			}
			node.Left = retHash
		} else {
			println(" RIGHT")
			retHash, err = t.set(node.Right, bits, depth+1, key, value, b)
			if err != nil {
				return nil, err
			}
			node.Right = retHash
		}
		// update the interior node
		newNodeBuf, err := node.encode()
		if err != nil {
			return nil, err
		}
		err = b.Put(node.hash(), newNodeBuf)
		if err != nil {
			return nil, err
		}
		return node.hash(), nil
	}
	return nil, errors.New("invalid node type")
}

func (t *Trie) emptyToLeaf(empty emptyNode, key []byte, data []byte, b *bolt.Bucket) ([]byte, error) {
	valueKey := sha256.Sum256(data)
	leaf := newLeafNode(empty.Prefix, key, valueKey[:])
	leafBuf, err := leaf.encode()
	if err != nil {
		return nil, err
	}

	// delete the empty node and store the leaf and the actual data
	if err := b.Delete(empty.hash(t.nonce)); err != nil {
		return nil, err
	}
	if err := b.Put(leaf.hash(t.nonce), leafBuf); err != nil {
		return nil, err
	}
	if err := b.Put(valueKey[:], data); err != nil {
		return nil, err
	}
	return leaf.hash(t.nonce), nil
}

// extendLeaf recursively extends a leaf node that's at the given prefix.
func (t *Trie) extendLeaf(currPrefix []bool, key1, valueKey1, key2, valueKey2 []byte, b *bolt.Bucket) ([]byte, []byte, error) {
	i := len(currPrefix)
	// TODO maybe we don't need to re-compute all the time
	fmt.Printf("EXTENDING LEAF %v\n", currPrefix)
	bits1 := toBinarySlice(key1)
	bits2 := toBinarySlice(key2)
	if bits1[i] != bits2[i] {
		// base case:
		left := newLeafNode(append(currPrefix, bits1[i]), key1, valueKey1)
		right := newLeafNode(append(currPrefix, bits2[i]), key2, valueKey2)
		leftBuf, err := left.encode()
		if err != nil {
			return nil, nil, err
		}
		rightBuf, err := right.encode()
		if err != nil {
			return nil, nil, err
		}
		if err := b.Put(left.hash(t.nonce), leftBuf); err != nil {
			return nil, nil, err
		}
		if err := b.Put(right.hash(t.nonce), rightBuf); err != nil {
			return nil, nil, err
		}
		fmt.Printf("BASE: STORING %x, %x, common %v\n", left.hash(t.nonce), right.hash(t.nonce), currPrefix)
		if bits1[i] {
			return left.hash(t.nonce), right.hash(t.nonce), nil
		}
		return right.hash(t.nonce), left.hash(t.nonce), nil
	}
	// recursive case:
	leftHash, rightHash, err := t.extendLeaf(append(currPrefix, bits1[i]), key1, valueKey1, key2, valueKey2, b)
	if err != nil {
		return nil, nil, err
	}

	interior := newInteriorNode(leftHash, rightHash)
	interiorBuf, err := interior.encode()
	if err != nil {
		return nil, nil, err
	}
	if err = b.Put(interior.hash(), interiorBuf); err != nil {
		return nil, nil, err
	}
	empty := newEmptyNode(append(currPrefix, !bits1[i]))
	emptyBuf, err := empty.encode()
	if err != nil {
		return nil, nil, err
	}
	if err = b.Put(empty.hash(t.nonce), emptyBuf); err != nil {
		return nil, nil, err
	}
	if bits1[i] {
		fmt.Printf("REC: STORING %x, %x\n", interior.hash(), empty.hash(t.nonce))
		return interior.hash(), empty.hash(t.nonce), nil
	}
	fmt.Printf("REC: STORING %x, %x\n", empty.hash(t.nonce), interior.hash())
	return empty.hash(t.nonce), interior.hash(), nil
}

// Delete deletes the key-value pair, an error is returned if the key does not
// exist.
func (t *Trie) Delete(key []byte) error {
	return t.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(t.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		rootKey := t.getRoot(b)
		if rootKey == nil {
			return errors.New("no root key")
		}
		return t.delete(key, b)
	})
}

// Get looks up whether a value exists for the given key.
func (t *Trie) Get(key []byte) ([]byte, error) {
	var val []byte
	err := t.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(t.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		rootKey := t.getRoot(b)
		if rootKey == nil {
			return errors.New("no root key")
		}
		var err error
		val, err = t.get(0, rootKey, toBinarySlice(key), key, b)
		return err
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (t *Trie) get(depth int, nodeKey []byte, bits []bool, key []byte, b *bolt.Bucket) ([]byte, error) {
	nodeVal := b.Get(nodeKey)
	if len(nodeVal) == 0 {
		return nil, errors.New("invalid node key")
	}
	switch nodeType(nodeVal[0]) {
	case typeEmpty:
		// base case 1
		fmt.Printf("-- EMPTY %x\n", nodeKey)
		return nil, nil
	case typeLeaf:
		// base case 2
		fmt.Printf("-- LEAF %x\n", nodeKey)
		node, err := decodeLeafNode(nodeVal)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(key, node.Key) {
			return nil, nil
		}
		return b.Get(node.DataKey), nil
	case typeInterior:
		// recursive case
		fmt.Printf("-- INTERIOR %x\n", nodeKey)
		node, err := decodeInteriorNode(nodeVal)
		if err != nil {
			return nil, err
		}
		if bits[depth] {
			return t.get(depth+1, node.Left, bits, key, b)
		}
		return t.get(depth+1, node.Right, bits, key, b)
	}
	return nil, errors.New("invalid node type")
}

func (t *Trie) delete(key []byte, b *bolt.Bucket) error {
	return nil
}

// getRaw gets the value, it returns nil if the value does not exist.
func (t *Trie) getRaw(key []byte) ([]byte, error) {
	var val []byte
	err := t.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(t.bucket)
		if b == nil {
			return errors.New("bucket does not exist")
		}
		buf := b.Get(key)
		val = make([]byte, len(buf))
		copy(val, buf)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

func genNonce() []byte {
	buf := make([]byte, 32)
	n, err := rand.Read(buf)
	if err != nil {
		return nil
	}
	if n != 32 {
		return nil
	}
	return buf
}

func toBinarySlice(buf []byte) []bool {
	bits := make([]bool, len(buf)*8)
	for i := 0; i < len(bits); i++ {
		bits[i] = (buf[i/8]<<uint(i%8))&(1<<7) > 0
	}
	return bits
}

func getByteSlice(bits []bool) []byte {
	buf := make([]byte, (len(bits)+7)/8)
	for i := 0; i < len(bits); i++ {
		if bits[i] {
			buf[i/8] |= (1 << 7) >> uint(i%8)
		}
	}
	return buf
}

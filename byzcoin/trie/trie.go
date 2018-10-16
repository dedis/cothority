package trie

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

const entryKey = "dedis_trie"
const nonceKey = "dedis_trie_nonce"

// Trie implements the merkle prefix tree described in the coniks paper.
type Trie struct {
	nonce []byte
	db    DB
	// We need to control the traversal during testing, so it's important
	// to have a way to specify an actual key for traversal instead of the
	// hash of it which we cannot predict. So we introduce the noHashKey
	// flag, which should only be used in the unit test.
	noHashKey bool
}

// NewTrie loads the tried from a boltDB database, it creates one if it does
// not exist.
func NewTrie(db DB) (Trie, error) {
	nonce := make([]byte, 32)
	err := db.Update(func(b bucket) error {
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
		return nil
	})

	if err != nil {
		return Trie{}, err
	}
	return Trie{
		nonce: nonce,
		db:    db,
	}, nil
}

// NewTrieWithNonce allows the user to specify the nonce, it will return an
// error if loading from an existing database.
func NewTrieWithNonce(db DB, nonce []byte) (Trie, error) {
	err := db.Update(func(b bucket) error {
		// create or load the nonce
		nonceBuf := b.Get([]byte(nonceKey))
		if nonceBuf != nil {
			return errors.New("nonce already exists")
		}
		if err := b.Put([]byte(nonceKey), nonce); err != nil {
			return err
		}

		// create the root node
		rootKey := b.Get([]byte(entryKey))
		if rootKey != nil {
			return errors.New("root already exists")
		}
		err := newRootNode(b, nonce)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Trie{}, err
	}
	return Trie{
		nonce: nonce,
		db:    db,
	}, nil
}

// newRootNode creates the root node and two empty nodes and store these in the
// bucket.
func newRootNode(b bucket, nonce []byte) error {
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

// GetRoot returns the root of the trie.
func (t *Trie) GetRoot() []byte {
	var root []byte
	t.db.View(func(b bucket) error {
		root = append([]byte{}, t.getRoot(b)...)
		return nil
	})
	return root
}

func (t *Trie) getRoot(b bucket) []byte {
	return b.Get([]byte(entryKey))
}

// Set sets or overwrites a key-value pair.
func (t *Trie) Set(key []byte, value []byte) error {
	return t.db.Update(func(b bucket) error {
		return t.startSet(key, value, b)
	})
}

// KVPair is the interface to get a
type KVPair interface {
	Op() OpType
	Key() []byte
	Val() []byte
}

// Batch is similar to Set, but for multiple key-value pairs.
func (t *Trie) Batch(pairs []KVPair) error {
	return t.db.Update(func(b bucket) error {
		for _, p := range pairs {
			switch p.Op() {
			case OpSet:
				if err := t.startSet(p.Key(), p.Val(), b); err != nil {
					return err
				}
			case OpDel:
				if err := t.startDel(p.Key(), b); err != nil {
					return err
				}
			default:
				return errors.New("no such operation")
			}
		}
		return nil
	})
}

func (t *Trie) startSet(key []byte, value []byte, b bucket) error {
	newRoot, err := t.set(t.getRoot(b), t.binSlice(key), 0, key, value, b)
	if err != nil {
		return err
	}
	return b.Put([]byte(entryKey), newRoot)
}

func (t *Trie) set(nodeKey []byte, bits []bool, depth int, key, value []byte, b bucket) ([]byte, error) {
	nodeVal := b.Get(nodeKey)
	if len(nodeVal) == 0 {
		return nil, errors.New("node key does not exist in set")
	}
	switch nodeType(nodeVal[0]) {
	case typeEmpty:
		// base case 1
		node, err := decodeEmptyNode(nodeVal)
		if err != nil {
			return nil, err
		}
		return t.emptyToLeaf(node, key, value, b)
	case typeLeaf:
		// base case 2
		node, err := decodeLeafNode(nodeVal)
		if err != nil {
			return nil, err
		}

		// If the key is the same, then we don't need to create a new
		// internal node, just update the value and hash.
		if bytes.Equal(node.Key, key) {
			if err := b.Delete(node.hash(t.nonce)); err != nil {
				return nil, err
			}
			node.Value = value
			leafBuf, err := node.encode()
			if err != nil {
				return nil, err
			}
			if err := b.Put(node.hash(t.nonce), leafBuf); err != nil {
				return nil, err
			}
			return node.hash(t.nonce), nil
		}
		// Otherwise, we need to create one or more interior nodes.
		bits1 := t.binSlice(node.Key)
		bits2 := t.binSlice(key)
		left, right, err := t.extendLeaf(node.Prefix, node.Key, node.Value, bits1, key, value, bits2, b)
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
		// Delete the old leaf node.
		if err := b.Delete(node.hash(t.nonce)); err != nil {
			return nil, err
		}
		return interior.hash(), nil
	case typeInterior:
		// recursive case
		node, err := decodeInteriorNode(nodeVal)
		if err != nil {
			return nil, err
		}
		oldHash := node.hash()
		var retHash []byte
		if bits[depth] {
			retHash, err = t.set(node.Left, bits, depth+1, key, value, b)
			if err != nil {
				return nil, err
			}
			node.Left = retHash
		} else {
			retHash, err = t.set(node.Right, bits, depth+1, key, value, b)
			if err != nil {
				return nil, err
			}
			node.Right = retHash
		}
		// update the interior node
		if err := b.Delete(oldHash); err != nil {
			return nil, err
		}
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

func (t *Trie) emptyToLeaf(empty emptyNode, key []byte, value []byte, b bucket) ([]byte, error) {
	leaf := newLeafNode(empty.Prefix, key, value)
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
	return leaf.hash(t.nonce), nil
}

// extendLeaf recursively extends a leaf node that's at the given prefix.
func (t *Trie) extendLeaf(currPrefix []bool,
	key1, valueKey1 []byte, bits1 []bool,
	key2, valueKey2 []byte, bits2 []bool,
	b bucket) ([]byte, []byte, error) {
	i := len(currPrefix)
	if bits1[i] != bits2[i] {
		// base case:
		currPrefixCopy := append([]bool{}, currPrefix...)
		left := newLeafNode(append(currPrefix, bits1[i]), key1, valueKey1)
		right := newLeafNode(append(currPrefixCopy, bits2[i]), key2, valueKey2)
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
		if bits1[i] {
			return left.hash(t.nonce), right.hash(t.nonce), nil
		}
		return right.hash(t.nonce), left.hash(t.nonce), nil
	}
	// recursive case:
	leftHash, rightHash, err := t.extendLeaf(append(currPrefix, bits1[i]), key1, valueKey1, bits1, key2, valueKey2, bits2, b)
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
		return interior.hash(), empty.hash(t.nonce), nil
	}
	return empty.hash(t.nonce), interior.hash(), nil
}

// Delete deletes the key-value pair, an error is returned if the key does not
// exist.
func (t *Trie) Delete(key []byte) error {
	return t.db.Update(func(b bucket) error {
		return t.startDel(key, b)
	})
}

func (t *Trie) startDel(key []byte, b bucket) error {
	rootKey := t.getRoot(b)
	if rootKey == nil {
		return errors.New("no root key")
	}
	newRoot, err := t.del(0, rootKey, t.binSlice(key), key, b)
	if err != nil {
		return err
	}
	if newRoot == nil {
		// nothing was deleted, so don't update the root
		return nil
	}
	return b.Put([]byte(entryKey), newRoot)
}

// Get looks up whether a value exists for the given key.
func (t *Trie) Get(key []byte) ([]byte, error) {
	var val []byte
	err := t.db.View(func(b bucket) error {
		rootKey := t.getRoot(b)
		if rootKey == nil {
			return errors.New("no root key")
		}
		var err error
		val, err = t.get(0, rootKey, t.binSlice(key), key, b)
		return err
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (t *Trie) get(depth int, nodeKey []byte, bits []bool, key []byte, b bucket) ([]byte, error) {
	nodeVal := b.Get(nodeKey)
	if len(nodeVal) == 0 {
		return nil, errors.New("node key does not exist in get")
	}
	switch nodeType(nodeVal[0]) {
	case typeEmpty:
		// base case 1
		return nil, nil
	case typeLeaf:
		// base case 2
		node, err := decodeLeafNode(nodeVal)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(key, node.Key) {
			return nil, nil
		}
		return node.Value, nil
	case typeInterior:
		// recursive case
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

// MakeEphemeralTrie creates a lazy ephemeral copy of the trie.
func (t *Trie) MakeEphemeralTrie() *EphemeralTrie {
	e := EphemeralTrie{
		source:     t,
		overlay:    make(map[string][]byte),
		deleteList: make(map[string][]byte),
		instrList:  nil,
	}
	return &e
}

// CopyTo will make a copy of the database, the target bucket will have
// its old data wiped.
func (t *Trie) CopyTo(target bucket) error {
	// First clean everything in the target bucket.
	err := target.ForEach(func(k, v []byte) error {
		return target.Delete(k)
	})
	if err != nil {
		return err
	}
	// Do the actual copy.
	return t.db.View(func(b bucket) error {
		rootKey := t.getRoot(b)
		if rootKey == nil {
			return errors.New("no root key")
		}
		// copy the well-known keys
		if err := target.Put([]byte(entryKey), append([]byte{}, rootKey...)); err != nil {
			return err
		}
		if err := target.Put([]byte(nonceKey), t.nonce); err != nil {
			return err
		}
		p := copyNodeProcessor{target}
		// copy the trie
		return t.dfs(&p, rootKey, b)
	})
}

// TODO for now we just replace leafs with empty nodes, which is ok but it'll
// be better if we can "shrink" the tree as well.
func (t *Trie) del(depth int, nodeKey []byte, bits []bool, key []byte, b bucket) ([]byte, error) {
	nodeVal := b.Get(nodeKey)
	if len(nodeVal) == 0 {
		return nil, errors.New("node key does not exist in del")
	}
	switch nodeType(nodeVal[0]) {
	case typeEmpty:
		// base case 1, nothing to delete
		return nil, nil
	case typeLeaf:
		node, err := decodeLeafNode(nodeVal)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(key, node.Key) {
			// key doesn't exist, nothing to delete
			return nil, nil
		}
		if err := b.Delete(node.hash(t.nonce)); err != nil {
			return nil, err
		}
		empty := newEmptyNode(node.Prefix)
		emptyBuf, err := empty.encode()
		if err != nil {
			return nil, err
		}
		if err := b.Put(empty.hash(t.nonce), emptyBuf); err != nil {
			return nil, err
		}
		return empty.hash(t.nonce), nil
	case typeInterior:
		node, err := decodeInteriorNode(nodeVal)
		if err != nil {
			return nil, err
		}
		// update this interior node
		if bits[depth] {
			// look left
			res, err := t.del(depth+1, node.Left, bits, key, b)
			if err != nil {
				return nil, err
			}
			if res == nil {
				// not found, so do nothing
				return nil, nil
			}
			// delete the old interior node
			if err := b.Delete(node.hash()); err != nil {
				return nil, err
			}
			// update this interior node
			node.Left = res
			nodeBuf, err := node.encode()
			if err != nil {
				return nil, err
			}
			return node.hash(), b.Put(node.hash(), nodeBuf)
		}
		// look right
		res, err := t.del(depth+1, node.Right, bits, key, b)
		if err != nil {
			return nil, err
		}
		if res == nil {
			// not found, so do nothing
			return nil, nil
		}
		// delete the old interior node
		if err := b.Delete(node.hash()); err != nil {
			return nil, err
		}
		// update this interior node
		node.Right = res
		nodeBuf, err := node.encode()
		if err != nil {
			return nil, err
		}
		return node.hash(), b.Put(node.hash(), nodeBuf)
	}
	return nil, errors.New("invalid node type")
}

// IsValid checks whether the trie is valid.
func (t *Trie) IsValid() error {
	p := countNodeProcessor{}
	err := t.db.View(func(b bucket) error {
		rootKey := t.getRoot(b)
		if rootKey == nil {
			return errors.New("no root key")
		}
		return t.dfs(&p, rootKey, b)
	})
	if err != nil {
		return err
	}

	// We can get proof for all the leaves.
	for _, leave := range p.leaves {
		proof, err := t.GetProof(leave.Key)
		if err != nil {
			return err
		}

		proof.noHashKey = t.noHashKey

		ok, err := proof.Exists(leave.Key)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("got absence proof")
		}
	}

	// Check that we have no dangling nodes.
	var total int
	err = t.db.View(func(b bucket) error {
		return b.ForEach(func(k, v []byte) error {
			total++
			return nil
		})
	})
	if err != nil {
		return err
	}
	if total != p.total+2 {
		// plus 2 because there are two well-known keys
		return errors.New("dangling nodes")
	}
	return nil
}

// getRaw gets the value, it returns nil if the value does not exist.
func (t *Trie) getRaw(key []byte) ([]byte, error) {
	var val []byte
	err := t.db.View(func(b bucket) error {
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

func (t *Trie) binSlice(buf []byte) []bool {
	if t.noHashKey {
		return toBinSlice(buf)
	}
	hashKey := sha256.Sum256(buf)
	return toBinSlice(hashKey[:])
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

func toBinSlice(buf []byte) []bool {
	bits := make([]bool, len(buf)*8)
	for i := 0; i < len(bits); i++ {
		bits[i] = (buf[i/8]<<uint(i%8))&(1<<7) > 0
	}
	return bits
}

func toByteSlice(bits []bool) []byte {
	buf := make([]byte, (len(bits)+7)/8)
	for i := 0; i < len(bits); i++ {
		if bits[i] {
			buf[i/8] |= (1 << 7) >> uint(i%8)
		}
	}
	return buf
}

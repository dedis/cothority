package patriciatrie

import (
	"bytes"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"
	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"golang.org/x/xerrors"
)

// PatriciaTrie implements the merkle patricia trie as implemented in Ethereum
// with the SHA3-256 hashing algorithm. The trie allows users to store key-value
// pairs using the `Put` and `Commit` methods and retrieving them using the `Get`
// method.
//
// A word about terminology: a `key` for users of this API translates to a `path`
// internally in this data structure. For instance, `[]byte("foo")`, represented
// in hex as `0x666f6f` would translate to the path `[0x06, 0x06, 0x06, 0x0f, 0x06, 0x0f]`
// where each element represents a nibble in the hex encoding
type PatriciaTrie struct {
	rootNode   node
	db         trie.DB
	uncommited []node
	rootKey    []byte

	sync.Mutex
}

var defaultRootKey = []byte{0x88, 0xc8, 0x88, 0x20, 0x9a, 0xa7, 0x89, 0x1b}

// NewPatriciaTrie creates a new PatriciaTrie with the root node
// stored in `defaultRootKey` key in the storage backend.
// It returns an error if the storage backend already has
// another value stored in the `defaultRootKey` key.
func NewPatriciaTrie(db trie.DB) (*PatriciaTrie, error) {
	return NewPatriciaTrieWithRootKey(db, defaultRootKey)
}

// NewPatriciaTrie creates a new PatriciaTrie with the root node
// stored in `rootKey` key in the storage backend.
// It returns an error if the storage backend already has
// another value stored in the `rootKey` key.
func NewPatriciaTrieWithRootKey(db trie.DB, rootKey []byte) (*PatriciaTrie, error) {
	err := db.View(func(b trie.Bucket) error {
		rootVal := b.Get(rootKey)
		if len(rootVal) != 0 {
			return xerrors.New("root key exists")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &PatriciaTrie{
		db:       db,
		rootNode: nil,
		rootKey:  rootKey,
	}, nil
}

// NewPatriciaTrie loads a PatriciaTrie with the root node
// stored in `defaultRootKey` key in the storage backend.
// It returns an error if the storage backend does not
// have a valid value stored in the `defaultRootKey` key.
func LoadPatriciaTrie(db trie.DB) (*PatriciaTrie, error) {
	return LoadPatriciaTrieWithRootKey(db, defaultRootKey)
}

// NewPatriciaTrie loads a PatriciaTrie with the root node
// stored in `rootKey` key in the storage backend.
// It returns an error if the storage backend does not
// have a valid value stored in the `rootKey` key.
func LoadPatriciaTrieWithRootKey(db trie.DB, rootKey []byte) (*PatriciaTrie, error) {
	t := &PatriciaTrie{db: db}
	var rootRef []byte
	err := db.View(func(b trie.Bucket) error {
		rootRef = b.Get(rootKey)
		if rootRef == nil {
			return xerrors.New("root not found in DB")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	n, _, err := t.decodeNodeReference(rootRef)
	if err != nil {
		return nil, err
	}
	t.rootNode = n
	return t, nil
}

// Get returns the value stored at the given `key` and
// returns an error if the `key` does not exist as a path
// in the trie.
func (p *PatriciaTrie) Get(key []byte) ([]byte, error) {
	p.Lock()
	defer p.Unlock()

	path := hexToNibbles(key)
	encodedValue, err := p.get(p.rootNode, path)
	if err != nil {
		return nil, err
	}
	elems, _, err := rlp.SplitList(encodedValue)
	if err != nil {
		return nil, err
	}
	value, _, err := rlp.SplitString(elems)
	if err != nil {
		return nil, err
	}
	return value, err
}

var keyNotFoundError = xerrors.New("key not found")

func (p *PatriciaTrie) get(n node, path []byte) (valuenode, error) {
	switch n.(type) {
	case branchNode:
		bnode := n.(branchNode)
		if len(path) == 0 {
			valnode := bnode[16].(valuenode)
			return valnode, nil
		}
		return p.get(bnode[path[0]], path[1:])
	case shortNode:
		snode := n.(shortNode)
		cp := commonPrefix(snode.remainingPath, path)
		if snode.flag < EVEN_LEAF {
			if cp != len(snode.remainingPath) {
				return nil, keyNotFoundError
			}
			return p.get(snode.value, path[cp:])
		}
		if !bytes.Equal(snode.remainingPath, path) {
			return nil, keyNotFoundError
		}
		valnode := snode.value.(valuenode)
		return valnode, nil
	default:
		return nil, keyNotFoundError
	}
}

// Put stores a `value` for a given `key` and returns
// an error if the operation fails.
// The changes are made in-memory and are not persisted
// to the storage backend as a result of this call.
func (p *PatriciaTrie) Put(key, value []byte) error {
	p.Lock()
	defer p.Unlock()

	path := hexToNibbles(key)
	// We embed the value in an array because indy stores
	// it like so. Maybe this should be handled by the caller?
	encodedValue, err := rlp.EncodeToBytes([]interface{}{value})
	if err != nil {
		return err
	}
	newRoot, err := p.put(p.rootNode, path, valuenode(encodedValue))
	if err != nil {
		return err
	}
	p.rootNode = newRoot
	return nil
}

func (p *PatriciaTrie) put(n node, path []byte, value valuenode) (node, error) {
	switch n.(type) {
	case nil:
		oddlen := len(path) % 2
		snode := shortNode{
			remainingPath: path,
			value:         value,
			flag:          shortNodeFlag(2 + oddlen),
		}
		p.uncommited = append(p.uncommited, snode)
		return snode, nil
	case branchNode:
		bnode := n.(branchNode)
		if len(path) == 0 {
			bnode[16] = value
			return bnode, nil
		}
		// recurse down
		newChild, err := p.put(bnode[path[0]], path[1:], value)
		if err != nil {
			return nil, err
		}
		bnode[path[0]] = newChild
		p.uncommited = append(p.uncommited, newChild, bnode)
		return bnode, nil
	case shortNode:
		snode := n.(shortNode)
		cp := commonPrefix(snode.remainingPath, path)
		if cp == len(snode.remainingPath) && cp <= len(path) {
			newChild, err := p.put(snode.value, path[cp:], value)
			if err != nil {
				return nil, err
			}
			snode.value = newChild
			oddlen := len(snode.remainingPath) % 2
			if _, ok := newChild.(valuenode); ok {
				snode.flag = shortNodeFlag(byte(2 + oddlen))
			} else {
				snode.flag = shortNodeFlag(byte(oddlen))
			}
			p.uncommited = append(p.uncommited, newChild, snode)
			return snode, nil
		}
		// new path diverges from here
		bnode := branchNode{}
		if cp < len(path) {
			oddlen := len(path[cp+1:]) % 2
			bnode[path[cp]] = shortNode{
				remainingPath: path[cp+1:],
				value:         value,
				flag:          shortNodeFlag(byte(2 + oddlen)),
			}
		} else {
			bnode[16] = value
		}

		oddlen := len(snode.remainingPath[cp+1:]) % 2
		newChild := shortNode{
			remainingPath: snode.remainingPath[cp+1:],
			value:         snode.value,
			flag:          shortNodeFlag(snode.isTerm() + byte(oddlen)),
		}
		bnode[snode.remainingPath[cp]] = newChild

		if len(snode.remainingPath[:cp]) > 0 {
			oddlen = len(snode.remainingPath[:cp]) % 2
			snode.remainingPath = snode.remainingPath[:cp]
			snode.value = bnode
			snode.flag = shortNodeFlag(byte(oddlen))
			p.uncommited = append(p.uncommited, newChild, bnode, snode)
			return snode, nil
		}
		p.uncommited = append(p.uncommited, newChild, bnode)
		return bnode, nil
	case valuenode:
		if len(path) == 0 {
			return value, nil
		}
		// branch node + leafnode
		bnode := branchNode{}
		bnode[16] = n
		oddlen := len(path[1:]) % 2
		newChild := shortNode{
			remainingPath: path[1:],
			value:         value,
			flag:          shortNodeFlag(byte(2 + oddlen)),
		}
		bnode[path[0]] = newChild
		p.uncommited = append(p.uncommited, newChild, bnode)
		return bnode, nil
	default:
		return nil, xerrors.New("invalid node type")
	}
}

func commonPrefix(a, b []byte) int {
	var i int
	for i = 0; i < len(a) && i < len(b) && a[i] == b[i]; i++ {
	}
	return i
}

func (p *PatriciaTrie) RootHash() []byte {
	hash := p.rootNode.hash()
	if hnode, ok := hash.(hashnode); ok {
		return hnode
	}
	rlpEncoded, err := rlp.EncodeToBytes(hash)
	if err != nil {
		panic(err)
	}
	return hashData(rlpEncoded)
}

// Commit walks over all the uncommitted nodes and
// persists them in the storage backend if required.
func (p *PatriciaTrie) Commit() error {
	p.Lock()
	defer p.Unlock()
	// TODO: This is O(n^2) since each call to hash is O(n)
	// Can be reduced to O(n) by caching the result of hash()
	// and making it available to nodes up the tree
	for _, n := range p.uncommited {
		hash := n.hash()
		// We persist something in the storage backend only
		// if its RLP encoding > 32 bytes. In the persistence layer,
		// the node is referenced as its SHA3-256 hash and the value is
		// its RLP encoding.
		if hnode, ok := hash.(hashnode); ok {
			rlpEncoding, err := rlp.EncodeToBytes(n)
			if err != nil {
				return err
			}
			err = p.db.Update(func(b trie.Bucket) error {
				return b.Put(hnode, rlpEncoding)
			})
			if err != nil {
				return err
			}
		}
	}
	// Root node needs to be persisted irrespective of its encoding length
	p.persistRoot()
	p.uncommited = make([]node, 0)
	return nil
}

func (p *PatriciaTrie) persistRoot() error {
	rlpEncoding, err := rlp.EncodeToBytes(p.rootNode)
	if err != nil {
		return err
	}
	hash := hashData(rlpEncoding)
	rlpEncodedHash, err := rlp.EncodeToBytes(hash)
	if err != nil {
		return err
	}
	err = p.db.Update(func(b trie.Bucket) error {
		return b.Put(p.rootKey, rlpEncodedHash)
	})

	return err
}

func (p *PatriciaTrie) decodeEncodedNode(encoding []byte) (node, error) {
	elems, _, err := rlp.SplitList(encoding)
	if err != nil {
		return nil, err
	}
	switch c, _ := rlp.CountValues(elems); c {
	case 2:
		// short node
		return p.decodeShortNode(elems)
	case 17:
		// branch node
		return p.decodeBranchNode(elems)
	default:
		return nil, xerrors.New("invalid node type")
	}
}

func (p *PatriciaTrie) decodeShortNode(encoding []byte) (node, error) {
	crp, reference, err := rlp.SplitString(encoding)
	if err != nil {
		return nil, err
	}
	remainingPath, flag := decodeCompact(crp)
	var value node
	if flag < 2 {
		value, _, err = p.decodeNodeReference(reference)
		if err != nil {
			return nil, err
		}
	} else {
		valueDecoded, _, err := rlp.SplitString(reference)
		if err != nil {
			return nil, err
		}
		value = valuenode(valueDecoded)
	}
	return shortNode{
		remainingPath: remainingPath,
		value:         value,
		flag:          flag,
	}, nil
}

func decodeCompact(path []byte) ([]byte, shortNodeFlag) {
	flag := shortNodeFlag(path[0])
	chop := 1
	if flag == EVEN_EXT || flag == EVEN_LEAF {
		chop = 2
	}
	nibbles := hexToNibbles(path)
	return nibbles[chop:], flag
}

func (p *PatriciaTrie) decodeNodeReference(ref []byte) (node, []byte, error) {
	kind, val, rest, err := rlp.Split(ref)
	if err != nil {
		return nil, nil, err
	}

	switch {
	case kind == rlp.List:
		// embedded
		if len(ref)-len(rest) > 32 {
			return nil, nil, xerrors.New("invalid embedded value")
		}
		n, err := p.decodeEncodedNode(ref)
		return n, rest, err
	case kind == rlp.String && len(val) == 0:
		// empty node
		return nil, rest, nil
	case kind == rlp.String && len(val) == 32:
		// hashnode: look it up in the db
		// TODO: maybe a lazy lookup might be more efficient
		var encoded []byte
		err := p.db.View(func(b trie.Bucket) error {
			encoded = b.Get(val)
			if len(encoded) == 0 {
				return xerrors.New("reference not found in db")
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
		n, err := p.decodeEncodedNode(encoded)
		if err != nil {
			return nil, nil, err
		}
		return n, rest, nil
	default:
		return nil, nil, xerrors.New("invalid reference type")
	}
}

func (p *PatriciaTrie) decodeBranchNode(encoded []byte) (node, error) {
	res := branchNode{}
	data := encoded
	for i := 0; i < 16; i++ {
		n, rest, err := p.decodeNodeReference(data)
		if err != nil {
			return nil, err
		}
		res[i] = n
		data = rest
	}

	val, _, err := rlp.SplitString(data)
	if err != nil {
		return nil, err
	}
	res[16] = valuenode(val)
	return res, nil
}

// GetAtRoot allows retrieving the value corresponding to a `key` when
// the rootHash of the trie was `rootHash`.
func (p *PatriciaTrie) GetAtRoot(rootHash, key []byte) ([]byte, error) {
	p.Lock()
	defer p.Unlock()

	encodedHash, err := rlp.EncodeToBytes(rootHash)
	if err != nil {
		return nil, err
	}
	rootNode, _, err := p.decodeNodeReference(encodedHash)
	if err != nil {
		return nil, err
	}
	path := hexToNibbles(key)
	encodedValue, err := p.get(rootNode, path)
	if err != nil {
		return nil, err
	}
	elems, _, err := rlp.SplitList(encodedValue)
	if err != nil {
		return nil, err
	}
	value, _, err := rlp.SplitString(elems)
	if err != nil {
		return nil, err
	}
	return value, err
}

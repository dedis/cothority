package trie

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
)

func (p *Proof) String() string {
	var out string
	out += fmt.Sprintf("Nonce: %x", p.Nonce)
	out += "\nInteriors:"
	for _, interior := range p.Interiors {
		out += fmt.Sprintf("\n\t%x -> [%x, %x]", interior.hash(), interior.Left, interior.Right)
	}
	out += fmt.Sprintf("\nLeaf: %x", p.Leaf.hash(p.Nonce))
	out += fmt.Sprintf("\nEmpty: %x", p.Empty.hash(p.Nonce))
	return out
}

// Exists checks the proof for inclusion/absence
func (p *Proof) Exists(key []byte) (bool, error) {
	if key == nil {
		return false, errors.New("key is nil")
	}

	if len(p.Interiors) == 0 {
		return false, errors.New("no interior nodes")
	}

	bits := p.binSlice(key)
	expectedHash := p.Interiors[0].hash() // first one is the root hash

	var i int
	for i = range p.Interiors {
		if !bytes.Equal(p.Interiors[i].hash(), expectedHash) {
			return false, errors.New("invalid hash chain")
		}
		if bits[i] {
			expectedHash = p.Interiors[i].Left
		} else {
			expectedHash = p.Interiors[i].Right
		}
	}
	if bytes.Equal(expectedHash, p.Leaf.hash(p.Nonce)) {
		if !equal(bits[:i+1], p.Leaf.Prefix) {
			return false, errors.New("invalid prefix in leaf node")
		}
		if !bytes.Equal(p.Leaf.Key, key) {
			return false, nil
		}
		return true, nil
	} else if bytes.Equal(expectedHash, p.Empty.hash(p.Nonce)) {
		if !equal(bits[:i+1], p.Empty.Prefix) {
			return false, errors.New("invalid prefix in empty node")
		}
		return false, nil
	} else {
		return false, errors.New("invalid edge node")
	}
}

// Match returns true if the proof is an existence proof for the given key, any
// error during the process of verifying the proof or if the key is absent then
// it returns false.
func (p *Proof) Match(key []byte) bool {
	ok, err := p.Exists(key)
	if err != nil {
		return false
	}
	return ok
}

// GetRoot returns the Merkle root.
func (p *Proof) GetRoot() []byte {
	if len(p.Interiors) == 0 {
		return nil
	}
	return p.Interiors[0].hash()
}

// KeyValue gets the key and the value that this proof contains. Similar to
// Match and Exists, the caller must check the key to make sure it is the one
// they're expecting.
func (p *Proof) KeyValue() ([]byte, []byte) {
	return p.Leaf.Key, p.Leaf.Value
}

// Key gets the key for this proof. Nil is returned if there is no key in the
// proof.
func (p *Proof) Key() []byte {
	return p.Leaf.Key
}

// Get returns the value associated with the given key in the proof. If the key
// does not exist, nil is returned. Note that there is at most one key/value
// pair in the proof.
func (p *Proof) Get(key []byte) []byte {
	if bytes.Equal(p.Leaf.Key, key) {
		return p.Leaf.Value
	}
	return nil
}

// GetProof gets the inclusion/absence proof for the given key.
func (t *Trie) GetProof(key []byte) (*Proof, error) {
	p := &Proof{}
	err := t.db.View(func(b Bucket) error {
		rootKey := t.getRoot(b)
		if rootKey == nil {
			return errors.New("no root key")
		}
		p.Nonce = clone(t.nonce)
		return t.getProof(0, rootKey, t.binSlice(key), p, b)
	})
	return p, err
}

// getProof updates Proof p as it traverses the tree.
func (t *Trie) getProof(depth int, nodeKey []byte, bits []bool, p *Proof, b Bucket) error {
	nodeVal := clone(b.Get(nodeKey))
	if len(nodeVal) == 0 {
		return errors.New("invalid node key")
	}
	switch nodeType(nodeVal[0]) {
	case typeEmpty:
		node, err := decodeEmptyNode(nodeVal)
		if err != nil {
			return err
		}
		p.Empty = node
		return nil
	case typeLeaf:
		node, err := decodeLeafNode(nodeVal)
		if err != nil {
			return err
		}
		p.Leaf = node
		return nil
	case typeInterior:
		node, err := decodeInteriorNode(nodeVal)
		if err != nil {
			return err
		}
		p.Interiors = append(p.Interiors, node)
		if bits[depth] {
			return t.getProof(depth+1, node.Left, bits, p, b)
		}
		// look right
		return t.getProof(depth+1, node.Right, bits, p, b)
	}
	return errors.New("invalid node type")
}

func (p *Proof) binSlice(buf []byte) []bool {
	if p.noHashKey {
		return toBinSlice(buf)
	}
	hashKey := sha256.Sum256(buf)
	return toBinSlice(hashKey[:])
}

func equal(a []bool, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

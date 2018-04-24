package collection

import (
	"crypto/sha256"
	"errors"
)

// Getter is the result of a get on a collection.
// It allows to get a record of the given key/value pair
// and prove, if that is the case, that the given key is in the collection.
type Getter struct {
	collection *Collection
	key        []byte
}

// Constructors

// Get returns a getter object, the result of a search of a given key on the collection.
// It takes as parameter the key we search in the collection.
func (c *Collection) Get(key []byte) Getter {
	return Getter{c, key}
}

// Methods

// Record returns a Record object that correspond to the result of the key search.
// The Record will contain a boolean "match" that is true if the search was successful and false otherwise.
func (g Getter) Record() (Record, error) {
	if len(g.key) == 0 {
		return Record{}, errors.New("cannot create a record with no key")
	}
	path := sha256.Sum256(g.key)

	depth := 0
	cursor := g.collection.root

	for {
		if !(cursor.known) {
			return Record{}, errors.New("record lies in an unknown subtree")
		}

		if cursor.leaf() {
			if equal(cursor.key, g.key) {
				return recordKeyMatch(g.collection, cursor), nil
			}
			return recordKeyMismatch(g.collection, g.key), nil
		}
		if bit(path[:], depth) {
			cursor = cursor.children.right
		} else {
			cursor = cursor.children.left
		}

		depth++
	}
}

// Proof returns a Proof of the presence or absence of a given key in the collection.
// The location the proof points to can contains the actual key.
// It can also contain another key, effectively proving that the key is absent from the collection.
func (g Getter) Proof() (Proof, error) {
	if len(g.key) == 0 {
		return Proof{}, errors.New("cannot create a proof with no key")
	}
	var proof Proof

	proof.collection = g.collection
	proof.Key = g.key

	proof.Root = dumpNode(g.collection.root)

	path := sha256.Sum256(g.key)

	depth := 0
	cursor := g.collection.root

	if !(cursor.known) {
		return proof, errors.New("record lies in unknown subtree")
	}

	for {
		if !(cursor.children.left.known) || !(cursor.children.right.known) {
			return proof, errors.New("record lies in unknown subtree")
		}

		proof.Steps = append(proof.Steps,
			step{dumpNode(cursor.children.left),
				dumpNode(cursor.children.right)})

		if bit(path[:], depth) {
			cursor = cursor.children.right
		} else {
			cursor = cursor.children.left
		}

		depth++

		if cursor.leaf() {
			break
		}
	}

	return proof, nil
}

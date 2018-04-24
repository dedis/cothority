package collection

import (
	"crypto/sha256"
	"errors"

	"github.com/dedis/protobuf"
)

// dump

type dump struct {
	Key    []byte   `protobuf:"opt"`
	Values [][]byte `protobuf:"opt"`

	Children struct {
		Left  [sha256.Size]byte
		Right [sha256.Size]byte
	}

	Label [sha256.Size]byte
}

// Constructors

func dumpNode(node *node) (dump dump) {
	dump.Label = node.label
	dump.Values = node.values

	if node.leaf() {
		dump.Key = node.key
	} else {
		dump.Children.Left = node.children.left.label
		dump.Children.Right = node.children.right.label
	}

	return
}

// Getters

func (d *dump) leaf() bool {
	var empty [sha256.Size]byte
	return (d.Children.Left == empty) && (d.Children.Right == empty)
}

// Methods

func (d *dump) consistent() bool {
	var toEncode toHash
	if d.leaf() {
		toEncode = toHash{true, d.Key, d.Values, [sha256.Size]byte{}, [sha256.Size]byte{}}
	} else {
		toEncode = toHash{false, []byte{}, d.Values, d.Children.Left, d.Children.Right}
	}

	return d.Label == toEncode.hash()
}

func (d *dump) to(node *node) {
	if !(node.known) && (node.label == d.Label) {
		node.known = true
		node.label = d.Label
		node.values = d.Values

		if d.leaf() {
			node.key = d.Key
		} else {
			node.branch()

			node.children.left.known = false
			node.children.right.known = false

			node.children.left.label = d.Children.Left
			node.children.right.label = d.Children.Right
		}
	}
}

// step

type step struct {
	Left  dump
	Right dump
}

// Proof

// Proof is an object representing the proof of presence or absence of a given key in a collection.
type Proof struct {
	collection *Collection
	key        []byte

	root  dump
	steps []step
}

// Getters

// Key returns the key the proof proves the presence or absence.
func (p Proof) Key() []byte {
	return p.key
}

// Methods

//Match returns true if the Proof asserts the presence of the key in the collection
// and false if it asserts its absence.
func (p Proof) Match() bool {
	if len(p.steps) == 0 {
		return false
	}

	path := sha256.Sum256(p.key)
	depth := len(p.steps) - 1

	if bit(path[:], depth) {
		return equal(p.key, p.steps[depth].Right.Key)
	}
	return equal(p.key, p.steps[depth].Left.Key)
}

// Values returns a copy of the values of the key which presence is proved by the Proof.
// It returns an error if the Proof proves the absence of the key.
func (p Proof) Values() ([]interface{}, error) {
	if len(p.steps) == 0 {
		return []interface{}{}, errors.New("proof has no steps")
	}

	path := sha256.Sum256(p.key)
	depth := len(p.steps) - 1

	match := false
	var rawValues [][]byte

	if bit(path[:], depth) {
		if equal(p.key, p.steps[depth].Right.Key) {
			match = true
			rawValues = p.steps[depth].Right.Values
		}
	} else {
		if equal(p.key, p.steps[depth].Left.Key) {
			match = true
			rawValues = p.steps[depth].Left.Values
		}
	}

	if !match {
		return []interface{}{}, errors.New("no match found")
	}

	if len(rawValues) != len(p.collection.fields) {
		return []interface{}{}, errors.New("wrong number of values")
	}

	var values []interface{}

	for index := 0; index < len(rawValues); index++ {
		value, err := p.collection.fields[index].Decode(rawValues[index])

		if err != nil {
			return []interface{}{}, err
		}

		values = append(values, value)
	}

	return values, nil
}

// Consistent returns true if the proof given is correctly set up.
func (p Proof) Consistent() bool {
	if len(p.steps) == 0 {
		return false
	}

	if !(p.root.consistent()) {
		return false
	}

	cursor := &(p.root)
	path := sha256.Sum256(p.key)

	for depth := 0; depth < len(p.steps); depth++ {
		if (cursor.Children.Left != p.steps[depth].Left.Label) || (cursor.Children.Right != p.steps[depth].Right.Label) {
			return false
		}

		if !(p.steps[depth].Left.consistent()) || !(p.steps[depth].Right.consistent()) {
			return false
		}

		if bit(path[:], depth) {
			cursor = &(p.steps[depth].Right)
		} else {
			cursor = &(p.steps[depth].Left)
		}
	}

	return cursor.leaf()
}

// collection

// Methods (collection) (serialization)

// Serialize serialize a proof.
// It transforms a given Proof into an array of byte, to allow easy exchange of proof, for example on a network.
func (c *Collection) Serialize(proof Proof) []byte {
	serializable := struct {
		Key   []byte
		Root  dump
		Steps []step
	}{proof.key, proof.root, proof.steps}

	buffer, _ := protobuf.Encode(&serializable)
	return buffer
}

// Deserialize is the inverse of serialize.
// It tansforms back a byte representation of a proof to a Proof object.
// It will generate an error if the given byte array doesn't represent a Proof.
func (c *Collection) Deserialize(buffer []byte) (Proof, error) {
	deserializable := struct {
		Key   []byte
		Root  dump
		Steps []step
	}{}

	err := protobuf.Decode(buffer, &deserializable)

	if err != nil {
		return Proof{}, err
	}

	return Proof{c, deserializable.Key, deserializable.Root, deserializable.Steps}, nil
}

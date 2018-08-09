package collection

import (
	"bytes"
	"crypto/sha256"
	"errors"

	"github.com/dedis/protobuf"
)

// Constructors

func dumpNode(n *node) (d dump) {
	// To avoid race conditions, we want a deep copy here.
	var nodeCopy node
	n.copyTo(&nodeCopy)

	d.Label = nodeCopy.label
	d.Values = nodeCopy.values

	if nodeCopy.leaf() {
		d.Key = nodeCopy.key
	} else {
		// label is an array, not a slice, so we don't need
		// do explicitly do a deep copy here.
		d.Children.Left = nodeCopy.children.left.label
		d.Children.Right = nodeCopy.children.right.label
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

// Getters

// TreeRootHash returns the hash of the merkle tree root.
func (p Proof) TreeRootHash() []byte {
	return p.Root.Label[:]
}

// Methods

//Match returns true if the Proof asserts the presence of the key in the collection
// and false if it asserts its absence.
func (p Proof) Match() bool {
	if len(p.Steps) == 0 {
		return false
	}

	path := sha256.Sum256(p.Key)
	depth := len(p.Steps) - 1

	if bit(path[:], depth) {
		return bytes.Equal(p.Key, p.Steps[depth].Right.Key)
	}
	return bytes.Equal(p.Key, p.Steps[depth].Left.Key)
}

// RawValues returns the raw values stored in the proof. This can be used if
// not the whole collection is present in the proof.
func (p Proof) RawValues() ([][]byte, error) {
	if len(p.Steps) == 0 {
		return [][]byte{}, errors.New("proof has no steps")
	}

	path := sha256.Sum256(p.Key)
	depth := len(p.Steps) - 1

	match := false
	var rawValues [][]byte

	if bit(path[:], depth) {
		if bytes.Equal(p.Key, p.Steps[depth].Right.Key) {
			match = true
			rawValues = p.Steps[depth].Right.Values
		}
	} else {
		if bytes.Equal(p.Key, p.Steps[depth].Left.Key) {
			match = true
			rawValues = p.Steps[depth].Left.Values
		}
	}

	if !match {
		return [][]byte{}, errors.New("no match found")
	}

	return rawValues, nil
}

// Values returns a copy of the values of the key which presence is proved by the Proof.
// It returns an error if the Proof proves the absence of the key.
func (p Proof) Values() ([]interface{}, error) {
	rawValues, err := p.RawValues()
	if err != nil {
		return []interface{}{}, err
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

// Consistent returns true if the given proof is correct, that is, if it is
// a valid representation and all steps are valid.
func (p Proof) Consistent() bool {
	if len(p.Steps) == 0 {
		return false
	}

	if !(p.Root.consistent()) {
		return false
	}

	cursor := &(p.Root)
	path := sha256.Sum256(p.Key)

	for depth := 0; depth < len(p.Steps); depth++ {
		if (cursor.Children.Left != p.Steps[depth].Left.Label) || (cursor.Children.Right != p.Steps[depth].Right.Label) {
			return false
		}

		if !(p.Steps[depth].Left.consistent()) || !(p.Steps[depth].Right.consistent()) {
			return false
		}

		if bit(path[:], depth) {
			cursor = &(p.Steps[depth].Right)
		} else {
			cursor = &(p.Steps[depth].Left)
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
	}{proof.Key, proof.Root, proof.Steps}

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

	return Proof{deserializable.Key, deserializable.Root, deserializable.Steps, c}, nil
}

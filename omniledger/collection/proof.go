package collection

import "errors"
import csha256 "crypto/sha256"
import "github.com/dedis/protobuf"

// dump

type dump struct {
	Label [csha256.Size]byte

	Key    []byte   `protobuf:"opt"`
	Values [][]byte `protobuf:"opt"`

	Children struct {
		Left  [csha256.Size]byte
		Right [csha256.Size]byte
	}
}

// Constructors

func dumpnode(node *node) (dump dump) {
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

func (this *dump) leaf() bool {
	var empty [csha256.Size]byte
	return (this.Children.Left == empty) && (this.Children.Right == empty)
}

// Methods

func (this *dump) consistent() bool {
	if this.leaf() {
		return this.Label == sha256(true, this.Key[:], this.Values)
	} else {
		return this.Label == sha256(false, this.Values, this.Children.Left[:], this.Children.Right[:])
	}
}

func (this *dump) to(node *node) {
	if !(node.known) && (node.label == this.Label) {
		node.known = true
		node.label = this.Label
		node.values = this.Values

		if this.leaf() {
			node.key = this.Key
		} else {
			node.branch()

			node.children.left.known = false
			node.children.right.known = false

			node.children.left.label = this.Children.Left
			node.children.right.label = this.Children.Right
		}
	}
}

// step

type step struct {
	Left  dump
	Right dump
}

// Proof

type Proof struct {
	collection *Collection
	key        []byte

	root  dump
	steps []step
}

// Getters

func (this Proof) Key() []byte {
	return this.key
}

// Methods

func (this Proof) Match() bool {
	if len(this.steps) == 0 {
		return false
	}

	path := sha256(this.key)
	depth := len(this.steps) - 1

	if bit(path[:], depth) {
		return equal(this.key, this.steps[depth].Right.Key)
	} else {
		return equal(this.key, this.steps[depth].Left.Key)
	}
}

func (this Proof) Values() ([]interface{}, error) {
	if len(this.steps) == 0 {
		return []interface{}{}, errors.New("Proof has no steps.")
	}

	path := sha256(this.key)
	depth := len(this.steps) - 1

	match := false
	var rawvalues [][]byte

	if bit(path[:], depth) {
		if equal(this.key, this.steps[depth].Right.Key) {
			match = true
			rawvalues = this.steps[depth].Right.Values
		}
	} else {
		if equal(this.key, this.steps[depth].Left.Key) {
			match = true
			rawvalues = this.steps[depth].Left.Values
		}
	}

	if !match {
		return []interface{}{}, errors.New("No match found.")
	}

	if len(rawvalues) != len(this.collection.fields) {
		return []interface{}{}, errors.New("Wrong number of values.")
	}

	var values []interface{}

	for index := 0; index < len(rawvalues); index++ {
		value, err := this.collection.fields[index].Decode(rawvalues[index])

		if err != nil {
			return []interface{}{}, err
		}

		values = append(values, value)
	}

	return values, nil
}

// Private methods

func (this Proof) consistent() bool {
	if len(this.steps) == 0 {
		return false
	}

	if !(this.root.consistent()) {
		return false
	}

	cursor := &(this.root)
	path := sha256(this.key)

	for depth := 0; depth < len(this.steps); depth++ {
		if (cursor.Children.Left != this.steps[depth].Left.Label) || (cursor.Children.Right != this.steps[depth].Right.Label) {
			return false
		}

		if !(this.steps[depth].Left.consistent()) || !(this.steps[depth].Right.consistent()) {
			return false
		}

		if bit(path[:], depth) {
			cursor = &(this.steps[depth].Right)
		} else {
			cursor = &(this.steps[depth].Left)
		}
	}

	return cursor.leaf()
}

// collection

// Methods (collection) (serialization)

func (this *Collection) Serialize(proof Proof) []byte {
	serializable := struct {
		Key   []byte
		Root  dump
		Steps []step
	}{proof.key, proof.root, proof.steps}

	buffer, _ := protobuf.Encode(&serializable)
	return buffer
}

func (this *Collection) Deserialize(buffer []byte) (Proof, error) {
	deserializable := struct {
		Key   []byte
		Root  dump
		Steps []step
	}{}

	error := protobuf.Decode(buffer, &deserializable)

	if error != nil {
		return Proof{}, error
	}

	return Proof{this, deserializable.Key, deserializable.Root, deserializable.Steps}, nil
}

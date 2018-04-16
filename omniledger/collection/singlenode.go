package collection

import (
	"crypto/sha256"
	"errors"
	"github.com/dedis/protobuf"
)

type toHash struct {
	IsLeaf bool
	Key    []byte
	Values [][]byte

	LeftLabel  [sha256.Size]byte
	RightLabel [sha256.Size]byte
}

// Private methods (collection) (single node operations)

func (c *Collection) update(node *node) error {
	if !(node.known) {
		return errors.New("updating an unknown node")
	}

	if !node.leaf() {
		if !(node.children.left.known) || !(node.children.right.known) {
			return errors.New("updating internal node with unknown children")
		}

		node.values = make([][]byte, len(c.fields))

		for index := 0; index < len(c.fields); index++ {
			parentValue, parentError := c.fields[index].Parent(node.children.left.values[index], node.children.right.values[index])

			if parentError != nil {
				return parentError
			}

			node.values[index] = parentValue
		}
	}

	label := node.generateHash()
	node.label = label

	return nil
}

func (c *Collection) setPlaceholder(node *node) error {
	node.known = true
	node.key = []byte{}
	node.values = make([][]byte, len(c.fields))

	for index := 0; index < len(c.fields); index++ {
		node.values[index] = c.fields[index].Placeholder()
	}

	node.children.left = nil
	node.children.right = nil

	err := c.update(node)
	if err != nil {
		return err
	}
	return nil
}

func (n *node) generateHash() [sha256.Size]byte {

	var toEncode toHash
	if n.leaf() {
		toEncode = toHash{true, n.key, n.values, [sha256.Size]byte{}, [sha256.Size]byte{}}
	} else {
		toEncode = toHash{false, []byte{}, n.values, n.children.left.label, n.children.right.label}
	}

	return toEncode.hash()
}

func (data *toHash) hash() [sha256.Size]byte {
	buff, err := protobuf.Encode(data)
	if err != nil {
		panic("couldn't encode: " + err.Error())
	}

	return sha256.Sum256(buff)
}

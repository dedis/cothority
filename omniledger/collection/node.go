package collection

import (
	"crypto/sha256"
)

//A node represents one element of the Merkle tree like data-structure.
type node struct {
	label [sha256.Size]byte

	known bool

	transaction struct {
		inconsistent bool
		backup       *node
	}

	key    []byte
	values [][]byte

	parent   *node
	children struct {
		left  *node
		right *node
	}
}

// Getters

func (n *node) root() bool {
	return n.parent == nil
}

func (n *node) leaf() bool {
	return n.children.left == nil
}

func (n *node) placeholder() bool {
	isLeaf := n.leaf()
	return isLeaf && (len(n.key) == 0)
}

// Methods

func (n *node) backup() {
	if n.transaction.backup == nil {
		n.transaction.backup = new(node)
		n.transaction.backup.overwrite(n)
		// For the backup we create a deep copy of the values.
		n.transaction.backup.values = make([][]byte, len(n.values))
		for i, v := range n.values {
			n.transaction.backup.values[i] = make([]byte, len(v))
			copy(n.transaction.backup.values[i], v)
		}
	}
}

// overwrite copies the fields from other into n. This is needed because
// node now has a mutex.
func (n *node) overwrite(other *node) {
	n.label = other.label
	n.known = other.known
	n.transaction.inconsistent = other.transaction.inconsistent

	n.key = other.key
	n.values = other.values

	n.parent = other.parent
	n.children.left = other.children.left
	n.children.right = other.children.right
}

func (n *node) restore() {
	if n.transaction.backup != nil {
		backup := n.transaction.backup
		n.overwrite(backup)
		n.transaction.backup = nil
	}
}

func (n *node) branch() {
	n.children.left = new(node)
	n.children.right = new(node)

	n.children.left.parent = n
	n.children.right.parent = n
}

func (n *node) prune() {
	n.children.left = nil
	n.children.right = nil
}

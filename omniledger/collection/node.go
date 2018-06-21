package collection

import "crypto/sha256"

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
	return n.leaf() && (len(n.key) == 0)
}

// Methods

func (n *node) backup() {
	if n.transaction.backup == nil {
		n.transaction.backup = new(node)

		n.transaction.backup.label = n.label
		n.transaction.backup.known = n.known
		n.transaction.backup.transaction.inconsistent = n.transaction.inconsistent

		n.transaction.backup.key = n.key
		n.transaction.backup.values = make([][]byte, len(n.values))
		copy(n.transaction.backup.values, n.values)

		n.transaction.backup.parent = n.parent
		n.transaction.backup.children.left = n.children.left
		n.transaction.backup.children.right = n.children.right
	}
}

func (n *node) restore() {
	if n.transaction.backup != nil {
		backup := n.transaction.backup
		(*n) = (*backup)
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

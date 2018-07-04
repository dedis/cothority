package collection

import (
	"crypto/sha256"
	"sync"
)

//A node represents one element of the Merkle tree like data-structure.
type node struct {
	sync.Mutex
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
	n.Lock()
	defer n.Unlock()
	return n.parent == nil
}

func (n *node) leaf() bool {
	n.Lock()
	defer n.Unlock()
	return n.children.left == nil
}

func (n *node) placeholder() bool {
	isLeaf := n.leaf()
	n.Lock()
	defer n.Unlock()
	return isLeaf && (len(n.key) == 0)
}

// Methods

func (n *node) backup() {
	n.Lock()
	defer n.Unlock()
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

func (n *node) copyKey() []byte {
	key := make([]byte, len(n.key))
	copy(key, n.key)
	return key
}

func (n *node) copyVal() [][]byte {
	// n.values == nil | n.Values[0]: []byte
	if n.values == nil {
		return nil
	}
	cv := make([][]byte, len(n.values))
	for i := 0; i < len(n.values); i++ {
		cv[i] = make([]byte, len(n.values[i]))
		copy(cv[i], n.values[i])
	}
	return cv
}

// overwrite copies the fields from other into n. This is needed because
// node now has a mutex.
func (n *node) overwrite(other *node) {
	n.label = other.label
	n.known = other.known
	n.transaction.inconsistent = other.transaction.inconsistent

	n.key = other.copyKey()
	n.values = other.copyVal()

	n.parent = other.parent
	n.children.left = other.children.left
	n.children.right = other.children.right
}

func (n *node) copyTo(dst *node) {
	n.Lock()
	dst.overwrite(n)
	n.Unlock()
}

func (n *node) restore() {
	n.Lock()
	defer n.Unlock()
	if n.transaction.backup != nil {
		backup := n.transaction.backup
		n.overwrite(backup)
		n.transaction.backup = nil
	}
}

func (n *node) branch() {
	n.Lock()
	defer n.Unlock()
	n.children.left = new(node)
	n.children.right = new(node)

	n.children.left.parent = n
	n.children.right.parent = n
}

func (n *node) prune() {
	n.Lock()
	defer n.Unlock()
	n.children.left = nil
	n.children.right = nil
}

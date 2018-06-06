package collection

import "crypto/sha256"

// Methods (collection) (transaction methods)

// Begin indicates the start of a transaction.
// It raises a flag preventing other transaction to take place on the same collection.
func (c *Collection) Begin() {
	if c.transaction.ongoing {
		panic("Transaction already in progress.")
	}

	c.transaction.ongoing = true
}

// Rollback cancels the transaction.
// It effectively replaces all nodes by their backup, if any and stops the transaction.
func (c *Collection) Rollback() {
	if !(c.transaction.ongoing) {
		panic("Transaction not in progress")
	}

	var explore func(*node)
	explore = func(node *node) {
		if node.transaction.inconsistent || (node.transaction.backup != nil) {
			node.restore()

			if !(node.leaf()) {
				explore(node.children.left)
				explore(node.children.right)
			}
		}
	}

	explore(c.root)

	c.transaction.id++
	c.transaction.ongoing = false
}

// End ends the transaction.
// It validates new states of nodes, fixes inconsistencies and increment the transaction id counter.
func (c *Collection) End() {
	if !(c.transaction.ongoing) {
		panic("Transaction not in progress.")
	}

	c.confirm()
	c.fix()

	if c.autoCollect.value {
		c.Collect()
	}

	c.transaction.id++
	c.transaction.ongoing = false
}

// Collect performs the garbage collection of the nodes out of the scope.
// It removes all nodes that are meant to be stored temporarily.
func (c *Collection) Collect() {
	var explore func(*node, [sha256.Size]byte, int)
	explore = func(node *node, path [sha256.Size]byte, bit int) {
		if !(node.known) {
			return
		}

		if bit > 0 && !(c.scope.match(path, bit-1)) {
			node.known = false
			node.key = []byte{}
			node.values = [][]byte{}

			node.prune()
		} else if !(node.leaf()) {
			setBit(path[:], bit+1, false)
			explore(node.children.left, path, bit+1)

			setBit(path[:], bit+1, true)
			explore(node.children.right, path, bit+1)
		}
	}

	if !(c.root.known) {
		return
	}

	var path [sha256.Size]byte
	none := true

	setBit(path[:], 0, false)
	if c.scope.match(path, 0) {
		none = false
	}

	setBit(path[:], 0, true)
	if c.scope.match(path, 0) {
		none = false
	}

	if none {
		c.root.known = false
		c.root.key = []byte{}
		c.root.values = [][]byte{}

		c.root.prune()
	} else {
		setBit(path[:], 0, false)
		explore(c.root.children.left, path, 0)

		setBit(path[:], 0, true)
		explore(c.root.children.right, path, 0)
	}
}

// Private methods (collection) (transaction methods)

func (c *Collection) confirm() {
	var explore func(*node)
	explore = func(node *node) {
		if node.transaction.inconsistent || (node.transaction.backup != nil) {
			node.transaction.backup = nil

			if !(node.leaf()) {
				explore(node.children.left)
				explore(node.children.right)
			}
		}
	}

	explore(c.root)
}

func (c *Collection) fix() {
	var explore func(*node)
	explore = func(node *node) {
		if node.transaction.inconsistent {
			if !(node.leaf()) {
				explore(node.children.left)
				explore(node.children.right)
			}

			c.update(node)
			node.transaction.inconsistent = false
		}
	}

	explore(c.root)
}

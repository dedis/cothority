package collection

import csha256 "crypto/sha256"

// Methods (collection) (transaction methods)

func (this *Collection) Begin() {
	if this.transaction.ongoing {
		panic("Transaction already in progress.")
	}

	this.transaction.ongoing = true
}

func (this *Collection) Rollback() {
	if !(this.transaction.ongoing) {
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

	explore(this.root)

	this.transaction.id++
	this.transaction.ongoing = false
}

func (this *Collection) End() {
	if !(this.transaction.ongoing) {
		panic("Transaction not in progress.")
	}

	this.confirm()
	this.fix()

	if this.autoCollect.value {
		this.Collect()
	}

	this.transaction.id++
	this.transaction.ongoing = false
}

func (this *Collection) Collect() {
	var explore func(*node, [csha256.Size]byte, int)
	explore = func(node *node, path [csha256.Size]byte, bit int) {
		if !(node.known) {
			return
		}

		if bit > 0 && !(this.scope.match(path, bit-1)) {
			node.known = false
			node.key = []byte{}
			node.values = [][]byte{}

			node.prune()
		} else if !(node.leaf()) {
			setbit(path[:], bit+1, false)
			explore(node.children.left, path, bit+1)

			setbit(path[:], bit+1, true)
			explore(node.children.right, path, bit+1)
		}
	}

	if !(this.root.known) {
		return
	}

	var path [csha256.Size]byte
	none := true

	setbit(path[:], 0, false)
	if this.scope.match(path, 0) {
		none = false
	}

	setbit(path[:], 0, true)
	if this.scope.match(path, 0) {
		none = false
	}

	if none {
		this.root.known = false
		this.root.key = []byte{}
		this.root.values = [][]byte{}

		this.root.prune()
	} else {
		setbit(path[:], 0, false)
		explore(this.root.children.left, path, 0)

		setbit(path[:], 0, true)
		explore(this.root.children.right, path, 0)
	}
}

// Private methods (collection) (transaction methods)

func (this *Collection) confirm() {
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

	explore(this.root)
}

func (this *Collection) fix() {
	var explore func(*node)
	explore = func(node *node) {
		if node.transaction.inconsistent {
			if !(node.leaf()) {
				explore(node.children.left)
				explore(node.children.right)
			}

			this.update(node)
			node.transaction.inconsistent = false
		}
	}

	explore(this.root)
}

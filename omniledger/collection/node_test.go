package collection

import "testing"

func TestNodeGetters(test *testing.T) {
	root := node{}
	left := node{}
	right := node{}

	root.children.left = &left
	root.children.right = &right

	left.parent = &root
	right.parent = &root

	if !(root.root()) {
		test.Error("[node.go]", "[root]", "root() returns false on root node.")
	}

	if left.root() || right.root() {
		test.Error("[node.go]", "[root]", "root() returns true on children nodes.")
	}

	if root.leaf() {
		test.Error("[node.go]", "[leaf]", "leaf() returns true on non-leaf node.")
	}

	if !(left.leaf()) || !(right.leaf()) {
		test.Error("[node.go]", "[leaf]", "leaf() returns false on leaf node.")
	}

	if root.placeholder() {
		test.Error("[node.go]", "[placeholder]", "placeholder() returns true on non-leaf node.")
	}

	if !(left.placeholder()) || !(right.placeholder()) {
		test.Error("[node.go]", "[placeholder]", "placeholder() returns false on placeholder node.")
	}

	left.key = []byte("leftkey")
	right.key = []byte("rightkey")

	if left.placeholder() || right.placeholder() {
		test.Error("[node.go]", "[placeholder]", "placeholder() returns true on non-placeholder leaf node.")
	}
}

func TestNodeBackupRestore(test *testing.T) {
	root := node{}
	root.key = []byte("mykey")

	root.label[0] = 11
	root.label[1] = 12

	root.children.left = new(node)
	root.children.right = new(node)
	root.children.left.key = []byte("leftkey")
	root.children.right.key = []byte("rightkey")

	root.backup()

	if root.transaction.backup == nil {
		test.Error("[node.go]", "[backup]", "backup() has no effect on non-previously backed up node.")
	}

	if !equal(root.transaction.backup.key, root.key) || (root.transaction.backup.label[0] != 11) || (root.transaction.backup.label[1] != 12) {
		test.Error("[node.go]", "[backup]", "backup() doesn't properly copy data to the backup item.")
	}

	root.key = []byte("myotherkey")
	root.label[0] = 0
	root.label[1] = 0

	root.children.left = nil
	root.children.right = nil

	root.backup()

	if equal(root.transaction.backup.key, root.key) || (root.transaction.backup.label[0] == 0) || (root.transaction.backup.label[1] == 0) {
		test.Error("[node.go]", "[backup]", "backup() is run again on a node that was already backed up.")
	}

	root.restore()

	if !equal(root.key, []byte("mykey")) || (root.label[0] != 11) || (root.label[1] != 12) {
		test.Error("[node.go]", "[restore]", "restore() does not restore values on previously backed up node.")
	}

	if root.transaction.backup != nil {
		test.Error("[node.go]", "[restore]", "restore() does not remove previous backup.")
	}

	if (root.children.left == nil) || (root.children.right == nil) {
		test.Error("[node.go]", "[restore]", "restore() does not restore children.")
	}

	if !equal(root.children.left.key, []byte("leftkey")) || !equal(root.children.right.key, []byte("rightkey")) {
		test.Error("[node.go]", "[restore]", "restore() does not correctly restore children.")
	}
}

func TestNodeBranchPrune(test *testing.T) {
	root := node{}
	root.branch()

	if (root.children.left == nil) || (root.children.right == nil) {
		test.Error("[node.go]", "[branch]", "Branch does not produce new children on a node.")
	}

	if (root.children.left.parent != &root) || (root.children.right.parent != &root) {
		test.Error("[node.go]", "[branch]", "Branch does not properly set children's parent.")
	}

	root.prune()

	if (root.children.left != nil) || (root.children.right != nil) {
		test.Error("[node.go]", "[prune]", "Prune does not remove a node's children.")
	}
}

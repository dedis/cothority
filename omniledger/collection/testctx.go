package collection

import "testing"
import csha256 "crypto/sha256"

// testctxstruct

type testctxstruct struct {
	file string
	test *testing.T

	verify testctxverifier
}

// Constructors

func testctx(file string, test *testing.T) testctxstruct {
	return testctxstruct{file, test, testctxverifier{file, test}}
}

// Methods

func (this testctxstruct) should_panic(prefix string, function func()) {
	defer func() {
		if recover() == nil {
			this.test.Error(this.file, prefix, "Function provided did not panic.")
		}
	}()

	function()
}

// testctxverifier

type testctxverifier struct {
	file string
	test *testing.T
}

// Methods

func (this testctxverifier) node(prefix string, collection *Collection, node *node) {
	if !(node.known) {
		return
	}

	if node.leaf() {
		if (node.children.left != nil) || (node.children.right != nil) {
			this.test.Error(this.file, prefix, "Leaf node has one or more children.")
			return
		}

		if node.label != sha256(true, node.key, node.values) {
			this.test.Error(this.file, prefix, "Wrong leaf node label.")
			return
		}
	} else {
		if (node.children.left == nil) || (node.children.right == nil) {
			this.test.Error(this.file, prefix, "Internal node is missing one or more children.")
			return
		}

		if (node.children.left.parent != node) || (node.children.right.parent != node) {
			this.test.Error(this.file, prefix, "Children of internal node don't have its parent correctly set.")
			return
		}

		if node.label != sha256(false, node.values, node.children.left.label[:], node.children.right.label[:]) {
			this.test.Error(this.file, prefix, "Wrong internal node label.")
			return
		}

		if node.children.left.known && node.children.right.known {
			for index := 0; index < len(collection.fields); index++ {
				parentvalue, parenterror := collection.fields[index].Parent(node.children.left.values[index], node.children.right.values[index])

				if parenterror != nil {
					this.test.Error(this.file, prefix, "Malformed children values.")
				}

				if !equal(parentvalue, node.values[index]) {
					this.test.Error(this.file, prefix, "One or more internal node values conflict with the corresponding children values.")
					return
				}
			}
		}
	}
}

func (this testctxverifier) treerecursion(prefix string, collection *Collection, node *node, path []bool) {
	this.node(prefix, collection, node)

	if node.leaf() {
		if !(node.placeholder()) {
			for index := 0; index < len(path); index++ {
				keyhash := sha256(node.key)
				if path[index] != bit(keyhash[:], index) {
					this.test.Error(this.file, prefix, "Leaf node on wrong path.")
				}
			}
		}
	} else {
		leftpath := make([]bool, len(path))
		rightpath := make([]bool, len(path))

		copy(leftpath, path)
		copy(rightpath, path)

		leftpath = append(leftpath, false)
		rightpath = append(rightpath, true)

		this.treerecursion(prefix, collection, node.children.left, leftpath)
		this.treerecursion(prefix, collection, node.children.right, rightpath)
	}
}

func (this testctxverifier) tree(prefix string, collection *Collection) {
	this.treerecursion(prefix, collection, collection.root, []bool{})
}

func (this testctxverifier) scoperecursion(prefix string, collection *Collection, node *node, path []bool) {
	if !(node.known) {
		return
	}

	var pathbuf [csha256.Size]byte

	for index := 0; index < len(path); index++ {
		setbit(pathbuf[:], index, path[index])
	}

	if node.known && len(path) > 1 && !(collection.scope.match(pathbuf, len(path)-2)) {
		this.test.Error(this.file, prefix, "Out-of-scope node was not pruned from tree.")
	} else {
		if !(node.leaf()) {
			leftpath := make([]bool, len(path))
			rightpath := make([]bool, len(path))

			copy(leftpath, path)
			copy(rightpath, path)

			leftpath = append(leftpath, false)
			rightpath = append(rightpath, true)

			this.scoperecursion(prefix, collection, node.children.left, leftpath)
			this.scoperecursion(prefix, collection, node.children.right, rightpath)
		}
	}
}

func (this testctxverifier) scope(prefix string, collection *Collection) {
	var pathbuf [csha256.Size]byte
	none := true

	setbit(pathbuf[:], 0, false)
	if collection.scope.match(pathbuf, 0) {
		none = false
	}

	setbit(pathbuf[:], 0, true)
	if collection.scope.match(pathbuf, 0) {
		none = false
	}

	if none {
		if collection.root.known {
			this.test.Error(this.file, prefix, "None-scope collection has known root.")
		}
	} else {
		if collection.root.known {
			this.scoperecursion(prefix, collection, collection.root.children.left, []bool{false})
			this.scoperecursion(prefix, collection, collection.root.children.right, []bool{true})
		}
	}
}

func (this testctxverifier) keyrecursion(key []byte, node *node) *node {
	if node.leaf() {
		if equal(node.key, key) {
			return node
		} else {
			return nil
		}
	} else {
		left := this.keyrecursion(key, node.children.left)
		right := this.keyrecursion(key, node.children.right)

		if left != nil {
			return left
		} else {
			return right
		}
	}
}

func (this testctxverifier) key(prefix string, collection *Collection, key []byte) {
	if this.keyrecursion(key, collection.root) == nil {
		this.test.Error(this.file, prefix, "Node not found.")
	}
}

func (this testctxverifier) nokey(prefix string, collection *Collection, key []byte) {
	if this.keyrecursion(key, collection.root) != nil {
		this.test.Error(this.file, prefix, "Unexpected node found.")
	}
}

func (this testctxverifier) values(prefix string, collection *Collection, key []byte, values ...interface{}) {
	node := this.keyrecursion(key, collection.root)

	if node == nil {
		this.test.Error(this.file, prefix, "Node not found.")
	}

	for index := 0; index < len(collection.fields); index++ {
		rawvalue := collection.fields[index].Encode(values[index])
		if !(equal(rawvalue, node.values[index])) {
			this.test.Error(this.file, prefix, "Wrong values.")
		}
	}
}

package collection

import (
	"crypto/sha256"
	"testing"
)

type testCtxStructure struct {
	file string
	test *testing.T

	verify testCtxVerifier
}

// Constructors

func testCtx(file string, test *testing.T) testCtxStructure {
	return testCtxStructure{file, test, testCtxVerifier{file, test}}
}

// Methods

func (t testCtxStructure) shouldPanic(prefix string, function func()) {
	defer func() {
		if recover() == nil {
			t.test.Error(t.file, prefix, "function provided did not panic")
		}
	}()

	function()
}

// testCtxVerifier

type testCtxVerifier struct {
	file string
	test *testing.T
}

// Methods

func (t testCtxVerifier) node(prefix string, collection *Collection, node *node) {
	if !(node.known) {
		return
	}

	if node.leaf() {
		if (node.children.left != nil) || (node.children.right != nil) {
			t.test.Error(t.file, prefix, "leaf node has one or more children")
			return
		}

		expectedLabel := node.generateHash()

		if node.label != expectedLabel {
			t.test.Error(t.file, prefix, "wrong leaf node label")
			return
		}
	} else {
		if (node.children.left == nil) || (node.children.right == nil) {
			t.test.Error(t.file, prefix, "internal node is missing one or more children")
			return
		}

		if (node.children.left.parent != node) || (node.children.right.parent != node) {
			t.test.Error(t.file, prefix, "children of internal node don't have its parent correctly set")
			return
		}

		expectedLabel := node.generateHash()

		if node.label != expectedLabel {
			t.test.Error(t.file, prefix, "wrong internal node label")
			return
		}

		if node.children.left.known && node.children.right.known {
			for index := 0; index < len(collection.fields); index++ {
				parentValue, parentError := collection.fields[index].Parent(node.children.left.values[index], node.children.right.values[index])

				if parentError != nil {
					t.test.Error(t.file, prefix, "malformed children values")
				}

				if !equal(parentValue, node.values[index]) {
					t.test.Error(t.file, prefix, "one or more internal node values conflict with the corresponding children values")
					return
				}
			}
		}
	}
}

func (t testCtxVerifier) treeRecursion(prefix string, collection *Collection, node *node, path []bool) {
	t.node(prefix, collection, node)

	if node.leaf() {
		if !(node.placeholder()) {
			for index := 0; index < len(path); index++ {
				keyHash := sha256.Sum256(node.key)
				if path[index] != bit(keyHash[:], index) {
					t.test.Error(t.file, prefix, "leaf node on wrong path")
				}
			}
		}
	} else {
		leftPath := make([]bool, len(path))
		rightPath := make([]bool, len(path))

		copy(leftPath, path)
		copy(rightPath, path)

		leftPath = append(leftPath, Left)
		rightPath = append(rightPath, Right)

		t.treeRecursion(prefix, collection, node.children.left, leftPath)
		t.treeRecursion(prefix, collection, node.children.right, rightPath)
	}
}

func (t testCtxVerifier) tree(prefix string, collection *Collection) {
	t.treeRecursion(prefix, collection, collection.root, []bool{})
}

func (t testCtxVerifier) scopeRecursion(prefix string, collection *Collection, node *node, path []bool) {
	if !(node.known) {
		return
	}

	var pathBuf [sha256.Size]byte

	for index := 0; index < len(path); index++ {
		setBit(pathBuf[:], index, path[index])
	}

	if node.known && len(path) > 1 && !(collection.scope.match(pathBuf, len(path)-2)) {
		t.test.Error(t.file, prefix, "out-of-scope node was not pruned from tree")
	} else {
		if !(node.leaf()) {
			leftPath := make([]bool, len(path))
			rightPath := make([]bool, len(path))

			copy(leftPath, path)
			copy(rightPath, path)

			leftPath = append(leftPath, Left)
			rightPath = append(rightPath, Right)

			t.scopeRecursion(prefix, collection, node.children.left, leftPath)
			t.scopeRecursion(prefix, collection, node.children.right, rightPath)
		}
	}
}

func (t testCtxVerifier) scope(prefix string, collection *Collection) {
	var pathBuf [sha256.Size]byte
	none := true

	setBit(pathBuf[:], 0, false)
	if collection.scope.match(pathBuf, 0) {
		none = false
	}

	setBit(pathBuf[:], 0, true)
	if collection.scope.match(pathBuf, 0) {
		none = false
	}

	if none {
		if collection.root.known {
			t.test.Error(t.file, prefix, "none-scope collection has known root")
		}
	} else {
		if collection.root.known {
			t.scopeRecursion(prefix, collection, collection.root.children.left, []bool{false})
			t.scopeRecursion(prefix, collection, collection.root.children.right, []bool{true})
		}
	}
}

func (t testCtxVerifier) keyRecursion(key []byte, node *node) *node {
	if node.leaf() {
		if equal(node.key, key) {
			return node
		}
		return nil
	}
	left := t.keyRecursion(key, node.children.left)
	right := t.keyRecursion(key, node.children.right)

	if left != nil {
		return left
	}
	return right
}

func (t testCtxVerifier) key(prefix string, collection *Collection, key []byte) {
	if t.keyRecursion(key, collection.root) == nil {
		t.test.Error(t.file, prefix, "node not found")
	}
}

func (t testCtxVerifier) noKey(prefix string, collection *Collection, key []byte) {
	if t.keyRecursion(key, collection.root) != nil {
		t.test.Error(t.file, prefix, "unexpected node found")
	}
}

func (t testCtxVerifier) values(prefix string, collection *Collection, key []byte, values ...interface{}) {
	node := t.keyRecursion(key, collection.root)

	if node == nil {
		t.test.Error(t.file, prefix, "node not found")
	}

	for index := 0; index < len(collection.fields); index++ {
		rawValue := collection.fields[index].Encode(values[index])
		if !(equal(rawValue, node.values[index])) {
			t.test.Error(t.file, prefix, "wrong values")
		}
	}
}

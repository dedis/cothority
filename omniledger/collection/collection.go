// Package collection is a Merkle-tree based data structure to securely and
// verifiably store key / value associations on untrusted nodes. The library
// in this package focuses on ease of use and flexibility, allowing to easily
// develop applications ranging from simple client-server storage to fully
// distributed and decentralized ledgers with minimal bootstrapping time.
package collection

// Collection represents the Merkle-tree based data structure.
// The data is defined by a pointer to its root.
type Collection struct {
	root   *node
	fields []Field
	scope  scope

	autoCollect flag
	transaction struct {
		ongoing bool
		id      uint64
	}
}

// Constructors

// New creates a new collection, with one root node and the given Fields
func New(fields ...Field) (collection Collection) {
	collection.fields = fields

	collection.scope.All()
	collection.autoCollect.Enable()

	collection.root = new(node)
	collection.root.known = true

	collection.root.branch()

	err := collection.setPlaceholder(collection.root.children.left)
	if err != nil {
		panic("error while generating left placeholder:" + err.Error())
	}
	err = collection.setPlaceholder(collection.root.children.right)
	if err != nil {
		panic("error while generating right placeholder:" + err.Error())
	}
	collection.update(collection.root)

	return
}

// NewVerifier creates a verifier. A verifier is defined as a collection that stores no data and no nodes.
// A verifiers is used to verify a query (i.e. that some data is or is not on the database).
func NewVerifier(fields ...Field) (verifier Collection) {
	verifier.fields = fields

	verifier.scope.None()
	verifier.autoCollect.Enable()

	empty := New(fields...)

	verifier.root = new(node)
	verifier.root.known = false
	verifier.root.label = empty.root.label

	return
}

// Methods

// Clone returns a deep copy of the collection.
// Note that the transaction id are restarted from 0 for the copy.
func (c *Collection) Clone() (collection Collection) {
	if c.transaction.ongoing {
		panic("Cannot clone a collection while a transaction is ongoing.")
	}

	collection.root = new(node)

	collection.fields = make([]Field, len(c.fields))
	copy(collection.fields, c.fields)

	collection.scope = c.scope.clone()
	collection.autoCollect = c.autoCollect

	collection.transaction.ongoing = false
	collection.transaction.id = 0

	var explore func(*node, *node)
	explore = func(dstCursor *node, srcCursor *node) {
		dstCursor.label = srcCursor.label
		dstCursor.known = srcCursor.known

		dstCursor.transaction.inconsistent = false
		dstCursor.transaction.backup = nil

		dstCursor.key = srcCursor.key
		dstCursor.values = make([][]byte, len(srcCursor.values))
		copy(dstCursor.values, srcCursor.values)

		if !(srcCursor.leaf()) {
			dstCursor.branch()
			explore(dstCursor.children.left, srcCursor.children.left)
			explore(dstCursor.children.right, srcCursor.children.right)
		}
	}

	explore(collection.root, c.root)

	return
}

// GetRoot returns the root hash of the collection, which cryptographically
// represents the whole set of key/value pairs in the collection.
func (c *Collection) GetRoot() []byte {
	return c.root.key
}

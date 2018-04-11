// Package collection is a Merkle-tree based data structure to securely and
// verifiably store key / value associations on untrusted nodes. The library
// in this package focuses on ease of use and flexibility, allowing to easily
// develop applications ranging from simple client-server storage to fully
// distributed and decentralized ledgers with minimal bootstrapping time.
package collection

// Collection
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

func New(fields ...Field) (collection Collection) {
	collection.fields = fields

	collection.scope.All()
	collection.autoCollect.Enable()

	collection.root = new(node)
	collection.root.known = true

	collection.root.branch()

	collection.placeholder(collection.root.children.left)
	collection.placeholder(collection.root.children.right)
	collection.update(collection.root)

	return
}

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

func (this *Collection) Clone() (collection Collection) {
	if this.transaction.ongoing {
		panic("Cannot clone a collection while a transaction is ongoing.")
	}

	collection.root = new(node)

	collection.fields = make([]Field, len(this.fields))
	copy(collection.fields, this.fields)

	collection.scope = this.scope.clone()
	collection.autoCollect = this.autoCollect

	collection.transaction.ongoing = false
	collection.transaction.id = 0

	var explore func(*node, *node)
	explore = func(dstcursor *node, srccursor *node) {
		dstcursor.label = srccursor.label
		dstcursor.known = srccursor.known

		dstcursor.transaction.inconsistent = false
		dstcursor.transaction.backup = nil

		dstcursor.key = srccursor.key
		dstcursor.values = make([][]byte, len(srccursor.values))
		copy(dstcursor.values, srccursor.values)

		if !(srccursor.leaf()) {
			dstcursor.branch()
			explore(dstcursor.children.left, srccursor.children.left)
			explore(dstcursor.children.right, srccursor.children.right)
		}
	}

	explore(collection.root, this.root)

	return
}

// GetRoot returns the root hash of the collection, which cryptographically
// represents the whole set of key/value pairs in the collection.
func (this *Collection) GetRoot() []byte {
	return this.root.key
}

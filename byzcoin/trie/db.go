package trie

// DB is the interface for the underlying storage system of the trie.
type DB interface {
	// Update executes a function within the context of a read-write
	// transaction. If no error is returned from the function then every
	// operation performed on the bucket is committed. If an error is
	// returned then nothing gets committed. Any error that is returned
	// from the function or returned from the commit is returned from the
	// method.
	Update(func(Bucket) error) error
	// View executes a function within the context of a read-only
	// transaction. Any error that is returned from the function is
	// returned from the method.
	View(func(Bucket) error) error
	// UpdateDryRun is similar to Update but the operations performed in
	// the function is never committed.
	UpdateDryRun(func(Bucket) error) error
	// Close releases all database resources. It will block waiting for any
	// open transactions to finish before closing the database and
	// returning.
	Close() error
}

// Bucket is the interface that enables raw operations on key/value pairs.
// It is invalid if it is used outside of a transaction, e.g., outside of
// DB.Update.
type Bucket interface {
	// Delete removes a key from the bucket. If the key does not exist
	// then nothing is done and a nil is returned. Returns an error if the
	// bucket was created from a read-only transaction.
	Delete([]byte) error
	// Put sets the value for a key in the bucket. If the key exist then
	// its previous value will be overwritten. Supplied value must remain
	// valid for the life of the transaction. Returns an error if an issue
	// occurs, e.g., the bucket was created from a read-only transaction.
	Put([]byte, []byte) error
	// Get retrieves the value for a key in the bucket. Returns a nil value
	// if the key does not exist or if the key is a nested bucket. The
	// returned value is only valid for the life of the transaction.
	Get([]byte) []byte
	// ForEach executes the given function for each key/value pair. If the
	// provided function returns an error then the iteration is stopped and
	// the error is returned to the caller.
	ForEach(func(k, v []byte) error) error
}

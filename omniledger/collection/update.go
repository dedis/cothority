package collection

import (
	"crypto/sha256"
	"errors"
)

// Interfaces

type userUpdate interface {
	Records() []Proof
	Check(ReadOnly) bool
	Apply(ReadWrite)
}

// ReadOnly defines a set of methods needed to have a read-only collection.
// It contains only a getter method, as it is the only method not modifying the collection.
type ReadOnly interface {
	Get([]byte) Record
}

// ReadWrite defines the set of methods needed to have a read-write collection.
type ReadWrite interface {
	Get([]byte) Record
	Add([]byte, ...interface{}) error
	Set([]byte, ...interface{}) error
	SetField([]byte, int, interface{}) error
	Remove([]byte) error
}

// Structs

// Update stores an update.
// That is a bunch of transactions to be applied on the same collection.
type Update struct {
	transaction uint64
	update      userUpdate
	proxy       proxy
}

// proxy to the actual collection for the getters.
type proxy struct {
	collection *Collection
	paths      map[[sha256.Size]byte]bool
}

// proxy

// Constructors

func (c *Collection) proxy(keys [][]byte) (proxy proxy) {
	proxy.collection = c
	proxy.paths = make(map[[sha256.Size]byte]bool)

	for index := 0; index < len(keys); index++ {
		proxy.paths[sha256.Sum256(keys[index])] = true
	}

	return
}

// Methods

func (p proxy) Get(key []byte) Record {
	if !(p.has(key)) {
		panic("accessing undeclared key from update")
	}

	record, _ := p.collection.Get(key).Record()
	return record
}

func (p proxy) Add(key []byte, values ...interface{}) error {
	if !(p.has(key)) {
		panic("accessing undeclared key from update")
	}

	return p.collection.Add(key, values...)
}

func (p proxy) Set(key []byte, values ...interface{}) error {
	if !(p.has(key)) {
		panic("accessing undeclared key from update")
	}

	return p.collection.Set(key, values...)
}

func (p proxy) SetField(key []byte, field int, value interface{}) error {
	if !(p.has(key)) {
		panic("accessing undeclared key from update")
	}

	return p.collection.SetField(key, field, value)
}

func (p proxy) Remove(key []byte) error {
	if !(p.has(key)) {
		panic("accessing undeclared key from update")
	}

	return p.collection.Remove(key)
}

// Private methods

func (p proxy) has(key []byte) bool {
	path := sha256.Sum256(key)
	return p.paths[path]
}

// collection

// Methods (collection) (update)

// Prepare prepares the userUpdate to do an update.
// It checks that every proof of the userUpdate is valid
// and then creates an Update object ready to apply the collection update.
func (c *Collection) Prepare(update userUpdate) (Update, error) {
	if c.root.transaction.inconsistent {
		panic("prepare() called on inconsistent root")
	}

	proofs := update.Records()
	keys := make([][]byte, len(proofs))

	for index := 0; index < len(proofs); index++ {
		if !(c.Verify(proofs[index])) {
			return Update{}, errors.New("invalid update: proof invalid")
		}

		keys[index] = proofs[index].Key
	}

	return Update{c.transaction.id, update, c.proxy(keys)}, nil
}

// Apply applies the update on the collection.
// If the update is not prepared, it will prepare it using the Prepare method.
func (c *Collection) Apply(object interface{}) error {
	switch update := object.(type) {
	case Update:
		return c.applyUpdate(update)
	case userUpdate:
		return c.applyUserUpdate(update)
	}

	panic("apply() only accepts Update objects or objects that implement the update interface")
}

// Private methods (collection) (update)

func (c *Collection) applyUpdate(update Update) error {
	if update.transaction != c.transaction.id {
		panic("update was not prepared during the current transaction")
	}

	if !(update.update.Check(update.proxy)) {
		return errors.New("update check failed")
	}

	if c.transaction.ongoing {
		update.update.Apply(update.proxy)
	} else {
		c.Begin()
		update.update.Apply(update.proxy)
		c.End()
	}

	return nil
}

func (c *Collection) applyUserUpdate(update userUpdate) error {
	preparedUpdate, err := c.Prepare(update)
	if err != nil {
		return err
	}

	return c.Apply(preparedUpdate)
}

package collection

import "errors"
import csha256 "crypto/sha256"

// Interfaces

type userupdate interface {
	Records() []Proof
	Check(ReadOnly) bool
	Apply(ReadWrite)
}

type ReadOnly interface {
	Get([]byte) Record
}

type ReadWrite interface {
	Get([]byte) Record
	Add([]byte, ...interface{}) error
	Set([]byte, ...interface{}) error
	SetField([]byte, int, interface{}) error
	Remove([]byte) error
}

// Structs

type Update struct {
	transaction uint64
	update      userupdate
	proxy       proxy
}

type proxy struct {
	collection *Collection
	paths      map[[csha256.Size]byte]bool
}

// proxy

// Constructors

func (this *Collection) proxy(keys [][]byte) (proxy proxy) {
	proxy.collection = this
	proxy.paths = make(map[[csha256.Size]byte]bool)

	for index := 0; index < len(keys); index++ {
		proxy.paths[sha256(keys[index])] = true
	}

	return
}

// Methods

func (this proxy) Get(key []byte) Record {
	if !(this.has(key)) {
		panic("Accessing undeclared key from update.")
	}

	record, _ := this.collection.Get(key).Record()
	return record
}

func (this proxy) Add(key []byte, values ...interface{}) error {
	if !(this.has(key)) {
		panic("Accessing undeclared key from update.")
	}

	return this.collection.Add(key, values...)
}

func (this proxy) Set(key []byte, values ...interface{}) error {
	if !(this.has(key)) {
		panic("Accessing undeclared key from update.")
	}

	return this.collection.Set(key, values...)
}

func (this proxy) SetField(key []byte, field int, value interface{}) error {
	if !(this.has(key)) {
		panic("Accessing undeclared key from update.")
	}

	return this.collection.SetField(key, field, value)
}

func (this proxy) Remove(key []byte) error {
	if !(this.has(key)) {
		panic("Accessing undeclared key from update.")
	}

	return this.collection.Remove(key)
}

// Private methods

func (this proxy) has(key []byte) bool {
	path := sha256(key)
	return this.paths[path]
}

// collection

// Methods (collection) (update)

func (this *Collection) Prepare(update userupdate) (Update, error) {
	if this.root.transaction.inconsistent {
		panic("Prepare() called on inconsistent root.")
	}

	proofs := update.Records()
	keys := make([][]byte, len(proofs))

	for index := 0; index < len(proofs); index++ {
		if !(this.Verify(proofs[index])) {
			return Update{}, errors.New("Invalid update: proof invalid.")
		}

		keys[index] = proofs[index].Key()
	}

	return Update{this.transaction.id, update, this.proxy(keys)}, nil
}

func (this *Collection) Apply(object interface{}) error {
	switch update := object.(type) {
	case Update:
		return this.applyupdate(update)
	case userupdate:
		return this.applyuserupdate(update)
	}

	panic("Apply() only accepts Update objects or objects that implement the update interface.")
}

// Private methods (collection) (update)

func (this *Collection) applyupdate(update Update) error {
	if update.transaction != this.transaction.id {
		panic("Update was not prepared during the current transaction.")
	}

	if !(update.update.Check(update.proxy)) {
		return errors.New("Update check failed.")
	}

	if this.transaction.ongoing {
		update.update.Apply(update.proxy)
	} else {
		this.Begin()
		update.update.Apply(update.proxy)
		this.End()
	}

	return nil
}

func (this *Collection) applyuserupdate(update userupdate) error {
	preparedupdate, err := this.Prepare(update)
	if err != nil {
		return err
	}

	return this.Apply(preparedupdate)
}

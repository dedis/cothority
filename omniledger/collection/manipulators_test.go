package collection

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func TestManipulatorsAdd(test *testing.T) {
	if testing.Short() {
		test.Skip("using race this takes a long time")
	}
	ctx := testCtx("[manipulators.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(rand.Uint32()))
		ctx.verify.tree("[stakecollection]", &collection)
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		ctx.verify.key("[stakecollection]", &collection, key)
	}

	unknownRoot := New()
	unknownRoot.root.known = false

	err := unknownRoot.Add([]byte("key"))
	if err == nil {
		test.Error("[manipulators.go]", "[unknownRoot]", "Add should yield an error on a collection with unknown root.")
	}

	unknownRootChildren := New()
	unknownRootChildren.root.children.left.known = false
	unknownRootChildren.root.children.right.known = false

	err = unknownRootChildren.Add([]byte("key"))
	if err == nil {
		test.Error("[manipulators.go]", "[unknownRootChildren]", "Add should yield an error on a collection with unknown root children.")
	}

	keyCollision := New()
	keyCollision.Add([]byte("key"))

	err = keyCollision.Add([]byte("key"))
	if err == nil {
		test.Error("[manipulators.go]", "[keyCollision]", "Add should yield an error on key collision.")
	}

	transaction := New(stake64)
	transaction.scope.None()
	transaction.transaction.ongoing = true

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		transaction.Add(key, uint64(rand.Uint32()))
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		ctx.verify.key("[transactioncollection]", &transaction, key)
	}

	ctx.shouldPanic("[wrongvalues]", func() {
		collection.Add([]byte("panickey"))
	})

	ctx.shouldPanic("[wrongvalues]", func() {
		keyCollision.Add([]byte("panickey"), uint64(13))
	})
}

func TestManipulatorsAddEmpty(test *testing.T) {
	collection := New(Data{})

	err := collection.Add([]byte{})
	require.NotNil(test, err)
}

func TestManipulatorsSet(test *testing.T) {
	if testing.Short() {
		test.Skip("using race this takes a long time")
	}
	ctx := testCtx("[manipulators.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index))
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Set(key, uint64(2*index))
		ctx.verify.tree("[stakecollection]", &collection)
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		ctx.verify.values("[set]", &collection, key, uint64(index*2))
	}

	unknownRoot := New(stake64)
	unknownRoot.root.known = false

	err := unknownRoot.Set([]byte("key"), uint64(0))
	if err == nil {
		test.Error("[manipulators.go]", "[unknownRoot]", "Set should yield an error on a collection with unknown root.")
	}

	unknownRootChildren := New(stake64)
	unknownRootChildren.root.children.left.known = false
	unknownRootChildren.root.children.right.known = false

	err = unknownRootChildren.Set([]byte("key"), uint64(0))
	if err == nil {
		test.Error("[manipulators.go]", "[unknownRootChildren]", "Set should yield an error on a collection with unknown root children.")
	}

	err = collection.Set([]byte("key"), uint64(13))
	if err == nil {
		test.Error("[manipulators.go]", "[notfound]", "Set should yield error when prompted to alter a value that does not exist.")
	}

	transaction := New(stake64)
	transaction.scope.None()
	transaction.transaction.ongoing = true

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		transaction.Add(key, uint64(index))
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		transaction.Set(key, uint64(2*index))
		transaction.Set(key, Same{})
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		ctx.verify.values("[transactioncollection]", &transaction, key, uint64(2*index))
	}

	ctx.shouldPanic("[wrongvalues]", func() {
		collection.Set([]byte("panickey"))
	})

	ctx.shouldPanic("[wrongvalues]", func() {
		collection.Set([]byte("panickey"), uint64(13), uint64(44))
	})
}

func TestManipulatorsSetField(test *testing.T) {
	ctx := testCtx("[manipulators.go]", test)

	data := Data{}
	collection := New(data, data)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, []byte{}, []byte{})
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.SetField(key, index%2, []byte("x"))
	}

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		if index%2 == 0 {
			ctx.verify.values("[setfield]", &collection, key, []byte("x"), []byte{})
		} else {
			ctx.verify.values("[setfield]", &collection, key, []byte{}, []byte("x"))
		}
	}

	ctx.shouldPanic("[fieldoutofrange]", func() {
		collection.SetField([]byte("key"), 5, []byte("data"))
	})
}

func TestManipulatorsRemove(test *testing.T) {
	if testing.Short() {
		test.Skip("using race this takes a long time")
	}
	ctx := testCtx("[manipulators.go]", test)

	collection := New()
	reference := New()

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key)
	}

	collection.Add([]byte("test"))
	reference.Add([]byte("test"))

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Remove(key)
		ctx.verify.tree("[remove]", &collection)
	}

	if collection.root.label != reference.root.label {
		test.Error("[manipulators.go]", "[remove]", "Label is not path-independent.")
	}

	unknownRoot := New()
	unknownRoot.root.known = false

	err := unknownRoot.Remove([]byte("key"))
	if err == nil {
		test.Error("[manipulators.go]", "[unknownRoot]", "Remove should yield an error on a collection with unknown root.")
	}

	unknownRootChildren := New()
	unknownRootChildren.root.children.left.known = false
	unknownRootChildren.root.children.right.known = false

	err = unknownRootChildren.Remove([]byte("key"))
	if err == nil {
		test.Error("[manipulators.go]", "[unknownRootChildren]", "Remove should yield an error on a collection with unknown root children.")
	}

	err = collection.Remove([]byte("wrongkey"))

	if err == nil {
		test.Error("[manipulators.go]", "[notfound]", "Remove should yield an error when provided with a key that does not lie in the collection.")
	}

	transaction := New()

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		transaction.Add(key)
	}

	transaction.scope.None()
	transaction.transaction.ongoing = true

	for index := 0; index < 512; index += 2 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		transaction.Remove(key)
	}

	for index := 0; index < 512; index += 2 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		ctx.verify.noKey("[transactioncollection]", &transaction, key)
	}

	for index := 1; index < 512; index += 2 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		ctx.verify.key("[transactioncollection]", &transaction, key)
	}

	for index := 1; index < 512; index += 2 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		transaction.Remove(key)
	}

	collision := func(key []byte, bits int) []byte {
		target := sha256.Sum256(key)
		sample := make([]byte, 8)

		for index := 0; ; index++ {
			binary.BigEndian.PutUint64(sample, uint64(index))
			hash := sha256.Sum256(sample)
			if match(hash[:], target[:], bits) {
				return sample
			}
		}
	}

	collisionKey := []byte("mykey")
	collidingKey := collision(collisionKey, 8)

	transaction.Add(collisionKey)
	transaction.Add(collidingKey)

	transaction.Remove(collisionKey)
	transaction.Remove(collidingKey)

	transaction.fix()
	transaction.Collect()
	transaction.transaction.ongoing = false

	empty := New()
	if transaction.root.label != empty.root.label {
		test.Error("[manipulators.go]", "[transaction]", "Transaction on collection doesn't produce empty root after removing all records.")
	}
}

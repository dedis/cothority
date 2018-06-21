package collection

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func TestTransactionBegin(test *testing.T) {
	ctx := testCtx("[transaction.go]", test)

	collection := New()
	collection.Begin()

	if !(collection.transaction.ongoing) {
		test.Error("[transaction.go]", "[begin]", "Begin() does not set the transaction flag.")
	}

	ctx.shouldPanic("[begin]", func() {
		collection.Begin()
	})
}

func TestTransactionRollback(test *testing.T) {
	ctx := testCtx("[transaction.go]", test)

	stake64 := Stake64{}

	collection := New(stake64)
	reference := New(stake64)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index))
		reference.Add(key, uint64(index))
	}

	collection.scope.None()
	reference.scope.None()

	collection.Begin()

	for index := 512; index < 1024; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index))
	}

	for index := 0; index < 1024; index += 3 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Set(key, uint64(3*index))
	}

	for index := 1; index < 1024; index += 3 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Remove(key)
	}

	idBefore := collection.transaction.id
	collection.Rollback()
	idAfter := collection.transaction.id

	if idAfter != idBefore+1 {
		test.Error("[transaction.go]", "[rollback]", "Rollback() does not increment the transaction id.")
	}

	ctx.verify.tree("[rollback]", &collection)

	if collection.root.label != reference.root.label {
		test.Error("[transaction.go]", "[rollback]", "Rollback() doesn't produce the same tree as before.")
	}

	collection.fix()

	if collection.root.label != reference.root.label {
		test.Error("[transaction.go]", "[rollback]", "Fixing after Rollback() has a non-null effect.")
	}

	noAutoCollect := New()
	noAutoCollect.autoCollect.Disable()
	noAutoCollect.scope.None()

	noAutoCollect.Begin()

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		noAutoCollect.Add(key)
	}

	noAutoCollect.End()

	if !(noAutoCollect.root.known) {
		test.Error("[transaction.go]", "[noAutoCollect]", "AutoCollect.Disable() seems to have no effect in preventing the collection of nodes after End().")
	}

	noAutoCollect.Collect()

	if noAutoCollect.root.known {
		test.Error("[transaction.go]", "[noAutoCollect]", "Collect() has no effect when AutoCollect is disabled.")
	}

	ctx.shouldPanic("[rollbackagain]", func() {
		collection.Rollback()
	})
}

func TestTransactionEnd(test *testing.T) {
	ctx := testCtx("[transaction.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	collection.Begin()

	for index := 0; index < 1024; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index))
	}

	for index := 0; index < 1024; index += 3 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Set(key, 3*uint64(index))
	}

	for index := 1; index < 1024; index += 3 {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Remove(key)
	}

	idbefore := collection.transaction.id
	collection.End()
	idafter := collection.transaction.id

	if idafter != idbefore+1 {
		test.Error("[transaction.go]", "[end]", "End() does not increment transaction id.")
	}

	ctx.verify.tree("[end]", &collection)

	for index := 0; index < 1024; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		if (index % 3) == 0 {
			ctx.verify.values("[end]", &collection, key, uint64(3*index))
		} else if (index % 3) == 1 {
			ctx.verify.noKey("[end]", &collection, key)
		} else {
			ctx.verify.values("[end]", &collection, key, uint64(index))
		}
	}

	oldroot := collection.root.label
	collection.fix()

	if collection.root.label != oldroot {
		test.Error("[transaction.go]", "[end]", "Fixing after End() alters the tree root.")
	}

	ctx.verify.scope("[scope]", &collection)

	ctx.shouldPanic("[endagain]", func() {
		collection.End()
	})
}

func TestTransactionCollect(test *testing.T) {
	ctx := testCtx("[transaction.go]", test)

	noneCollection := New()
	noneCollection.scope.None()

	noneCollection.root.children.left.branch()
	noneCollection.root.children.right.branch()
	noneCollection.root.children.right.children.left.branch()
	noneCollection.root.children.right.children.right.branch()

	noneCollection.Collect()

	if noneCollection.root.known {
		test.Error("[transaction.go]", "[root]", "Root is known after collecting collection with empty scope.")
	}

	if (noneCollection.root.children.left) != nil || (noneCollection.root.children.right) != nil {
		test.Error("[transaction.go]", "[children]", "Children of root are not pruned after collecting collection with empty scope.")
	}

	collection := New()
	collection.scope.Add([]byte{0x00}, 1)
	collection.scope.Add([]byte{0xff}, 3)
	collection.scope.Add([]byte{0xd2}, 6)

	collection.transaction.ongoing = true

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key)
	}

	collection.fix()
	collection.Collect()
	collection.transaction.ongoing = false

	ctx.verify.scope("[collect]", &collection)

	unknownRoot := New()
	unknownRoot.root.known = false
	unknownRoot.Collect()

	if (unknownRoot.root.children.left == nil) || (unknownRoot.root.children.right == nil) {
		test.Error("[transaction.go]", "[unknownRoot]", "Collect() removes children of unknown root.")
	}

	collection.scope.None()
	collection.scope.Add([]byte{0xd2}, 6)
	collection.root.children.left.known = false
	collection.Collect()

	if (collection.root.children.left.children.left == nil) || (collection.root.children.left.children.right == nil) {
		test.Error("[transaction.go]", "[unknownrootchild]", "Collect() removes children of unknown root child.")
	}
}

func TestTransactionConfirm(test *testing.T) {
	collection := New()
	reference := New()

	collection.transaction.ongoing = true
	reference.transaction.ongoing = true

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key)
		reference.Add(key)
	}

	var explore func(*node) int
	explore = func(node *node) int {
		if node.leaf() {
			if node.transaction.backup != nil {
				return 1
			}
			return 0
		}
		offset := 0
		if node.transaction.backup != nil {
			offset = 1
		}
		return offset + explore(node.children.left) + explore(node.children.right)
	}

	count := explore(collection.root)
	if count < 512 {
		test.Error("[transaction.go]", "[backup]", "Not enough backups after transaction operations.")
	}

	collection.confirm()

	count = explore(collection.root)
	if count != 0 {
		test.Error("[transaction.go]", "[confirm]", "confirm() does not remove all the backups.")
	}

	collection.fix()
	reference.fix()

	if collection.root.label != reference.root.label {
		test.Error("[transaction.go]", "[confirm]", "confirm() does not only remove the backups, but it also alters the values of the nodes.")
	}
}

func TestTransactionFix(test *testing.T) {
	ctx := testCtx("[transaction.go]", test)

	collection := New()

	// generate key with hash with Left direction as first bit
	collection.root.children.left.key = []byte{0x00}
	for {
		collection.root.children.left.key[0]++
		hash := sha256.Sum256(collection.root.children.left.key[:])
		if bit(hash[:], 0) == Left {
			break
		}
	}

	collection.root.children.left.transaction.inconsistent = true
	collection.root.transaction.inconsistent = true

	collection.fix()
	ctx.verify.tree("[fix]", &collection)

	oldRootLabel := collection.root.label

	// generate key with hash with Right direction as first bit
	collection.root.children.right.key = []byte{0x00}
	for {
		collection.root.children.right.key[0]++
		hash := sha256.Sum256(collection.root.children.right.key[:])
		if bit(hash[:], 0) == Right {
			break
		}
	}

	collection.root.children.right.transaction.inconsistent = true

	collection.fix()

	if collection.root.label != oldRootLabel {
		test.Error("[transaction.go]", "[fix]", "Fix should not visit nodes that are not marked as inconsistent.")
	}

	collection.root.transaction.inconsistent = true
	collection.fix()

	if collection.root.label == oldRootLabel {
		test.Error("[transaction.go]", "[fix]", "Fix should alter the label of the root of a collection tree.")
	}

	ctx.verify.tree("[fix]", &collection)
}

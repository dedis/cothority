package collection

import "testing"
import "encoding/binary"

func TestTransactionBegin(test *testing.T) {
	ctx := testctx("[transaction.go]", test)

	collection := New()
	collection.Begin()

	if !(collection.transaction.ongoing) {
		test.Error("[transaction.go]", "[begin]", "Begin() does not set the transaction flag.")
	}

	ctx.should_panic("[begin]", func() {
		collection.Begin()
	})
}

func TestTransactionRollback(test *testing.T) {
	ctx := testctx("[transaction.go]", test)

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

	idbefore := collection.transaction.id
	collection.Rollback()
	idafter := collection.transaction.id

	if idafter != idbefore+1 {
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

	noautocollect := New()
	noautocollect.autoCollect.Disable()
	noautocollect.scope.None()

	noautocollect.Begin()

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		noautocollect.Add(key)
	}

	noautocollect.End()

	if !(noautocollect.root.known) {
		test.Error("[transaction.go]", "[noautocollect]", "AutoCollect.Disable() seems to have no effect in preventing the collection of nodes after End().")
	}

	noautocollect.Collect()

	if noautocollect.root.known {
		test.Error("[transaction.go]", "[noautocollect]", "Collect() has no effect when AutoCollect is disabled.")
	}

	ctx.should_panic("[rollbackagain]", func() {
		collection.Rollback()
	})
}

func TestTransactionEnd(test *testing.T) {
	ctx := testctx("[transaction.go]", test)

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
			ctx.verify.nokey("[end]", &collection, key)
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

	ctx.should_panic("[endagain]", func() {
		collection.End()
	})
}

func TestTransactionCollect(test *testing.T) {
	ctx := testctx("[transaction.go]", test)

	nonecollection := New()
	nonecollection.scope.None()

	nonecollection.root.children.left.branch()
	nonecollection.root.children.right.branch()
	nonecollection.root.children.right.children.left.branch()
	nonecollection.root.children.right.children.right.branch()

	nonecollection.Collect()

	if nonecollection.root.known {
		test.Error("[transaction.go]", "[root]", "Root is known after collecting collection with empty scope.")
	}

	if (nonecollection.root.children.left) != nil || (nonecollection.root.children.right) != nil {
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

	unknownroot := New()
	unknownroot.root.known = false
	unknownroot.Collect()

	if (unknownroot.root.children.left == nil) || (unknownroot.root.children.right == nil) {
		test.Error("[transaction.go]", "[unknownroot]", "Collect() removes children of unknown root.")
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
			} else {
				return 0
			}
		} else {
			if node.transaction.backup != nil {
				return 1 + explore(node.children.left) + explore(node.children.right)
			} else {
				return explore(node.children.left) + explore(node.children.right)
			}
		}
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
	ctx := testctx("[transaction.go]", test)

	collection := New()

	collection.root.children.left.key = []byte("leftkey")
	collection.root.children.left.transaction.inconsistent = true
	collection.root.transaction.inconsistent = true

	collection.fix()
	ctx.verify.tree("[fix]", &collection)

	oldrootlabel := collection.root.label

	collection.root.children.right.key = []byte("rightkey")
	collection.root.children.right.transaction.inconsistent = true

	collection.fix()

	if collection.root.label != oldrootlabel {
		test.Error("[transaction.go]", "[fix]", "Fix should not visit nodes that are not marked as inconsistent.")
	}

	collection.root.transaction.inconsistent = true
	collection.fix()

	if collection.root.label == oldrootlabel {
		test.Error("[transaction.go]", "[fix]", "Fix should alter the label of the root of a collection tree.")
	}

	ctx.verify.tree("[fix]", &collection)
}

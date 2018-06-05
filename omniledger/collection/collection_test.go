package collection

import (
	"encoding/binary"
	"testing"
)

func TestCollectionEmptyCollection(test *testing.T) {
	ctx := testCtx("[collection.go]", test)

	baseCollection := New()

	if !(baseCollection.autoCollect.value) {
		test.Error("[collection.go]", "[autocollect]", "AutoCollect does not have true as default value.")
	}

	if !(baseCollection.root.known) || !(baseCollection.root.children.left.known) || !(baseCollection.root.children.right.known) {
		test.Error("[collection.go]", "[known]", "New collection has unknown nodes.")
	}

	if !(baseCollection.root.root()) {
		test.Error("[collection.go]", "[root]", "Collection root is not a root.")
	}

	if baseCollection.root.leaf() {
		test.Error("[collection.go]", "[root]", "Collection root doesn't have children.")
	}

	if !(baseCollection.root.children.left.placeholder()) || !(baseCollection.root.children.right.placeholder()) {
		test.Error("[collection.go]", "[leaves]", "Collection leaves are not placeholder leaves.")
	}

	if len(baseCollection.root.values) != 0 || len(baseCollection.root.children.left.values) != 0 || len(baseCollection.root.children.right.values) != 0 {
		test.Error("[collection.go]", "[values]", "Nodes of a collection without fields have values.")
	}

	ctx.verify.tree("[baseCollection]", &baseCollection)

	stake64 := Stake64{}
	stakeCollection := New(stake64)

	if len(stakeCollection.root.values) != 1 || len(stakeCollection.root.children.left.values) != 1 || len(stakeCollection.root.children.right.values) != 1 {
		test.Error("[collection.go]", "[values]", "Nodes of a stake collection don't have exactly one value.")
	}

	rootStake, rootError := stake64.Decode(stakeCollection.root.values[0])

	if rootError != nil {
		test.Error("[collection.go]", "[stake]", "Malformed stake root value.")
	}

	leftStake, leftError := stake64.Decode(stakeCollection.root.children.left.values[0])

	if leftError != nil {
		test.Error("[collection.go]", "[stake]", "Malformed stake left child value.")
	}

	rightStake, rightError := stake64.Decode(stakeCollection.root.children.right.values[0])

	if rightError != nil {
		test.Error("[collection.go]", "[stake]", "Malformed stake right child value")
	}

	if rootStake.(uint64) != 0 || leftStake.(uint64) != 0 || rightStake.(uint64) != 0 {
		test.Error("[collection.go]", "[stake]", "Nodes of an empty stake collection don't have zero stake.")
	}

	ctx.verify.tree("[stakeCollection]", &stakeCollection)

	data := Data{}
	stakeDataCollection := New(stake64, data)

	if len(stakeDataCollection.root.values) != 2 || len(stakeDataCollection.root.children.left.values) != 2 || len(stakeDataCollection.root.children.right.values) != 2 {
		test.Error("[collection.go]", "[values]", "Nodes of a data and stake collection don't have exactly one value.")
	}

	if len(stakeDataCollection.root.values[1]) != 0 || len(stakeDataCollection.root.children.left.values[1]) != 0 || len(stakeDataCollection.root.children.right.values[1]) != 0 {
		test.Error("[collection.go]", "[values]", "Nodes of a data and stake collection don't have empty data value.")
	}

	ctx.verify.tree("[stakeDataCollection]", &stakeDataCollection)
}

func TestCollectionEmptyVerifier(test *testing.T) {
	baseCollection := New()
	baseVerifier := NewVerifier()

	if !(baseVerifier.autoCollect.value) {
		test.Error("[collection.go]", "[autocollect]", "AutoCollect does not have true as default value.")
	}

	if baseVerifier.root.known {
		test.Error("[collection.go]", "[known]", "Empty verifier has known root.")
	}

	if (baseVerifier.root.children.left != nil) || (baseVerifier.root.children.right != nil) {
		test.Error("[collection.go]", "[root]", "Empty verifier root has children.")
	}

	if baseVerifier.root.label != baseCollection.root.label {
		test.Error("[collection.go]", "[Label]", "Wrong verifier label.")
	}

	stake64 := Stake64{}

	stakeCollection := New(stake64)
	stakeVerifier := NewVerifier(stake64)

	if stakeVerifier.root.known {
		test.Error("[collection.go]", "[known]", "Empty stake verifier has known root.")
	}

	if (stakeVerifier.root.children.left != nil) || (stakeVerifier.root.children.right != nil) {
		test.Error("[collection.go]", "[root]", "Empty stake verifier root has children.")
	}

	if stakeVerifier.root.label != stakeCollection.root.label {
		test.Error("[collection.go]", "[Label]", "Wrong stake verifier label.")
	}

	data := Data{}

	stakeDataCollection := New(stake64, data)
	stakeDataVerifier := NewVerifier(stake64, data)

	if stakeDataVerifier.root.known {
		test.Error("[collection.go]", "[known]", "Empty stake and data verifier has known root.")
	}

	if (stakeDataVerifier.root.children.left != nil) || (stakeDataVerifier.root.children.right != nil) {
		test.Error("[collection.go]", "[root]", "Empty stake and data verifier root has children.")
	}

	if stakeDataVerifier.root.label != stakeDataCollection.root.label {
		test.Error("[collection.go]", "[Label]", "Wrong stake and data verifier label.")
	}
}

func TestCollectionClone(test *testing.T) {
	ctx := testCtx("[collection.go]", test)

	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index), key)
	}

	clone := collection.Clone()

	ctx.verify.tree("[clone]", &clone)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		ctx.verify.values("[clone]", &clone, key, uint64(index), key)
	}

	ctx.shouldPanic("[clone]", func() {
		collection.Begin()
		collection.Clone()
		collection.End()
	})
}

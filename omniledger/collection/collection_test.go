package collection

import "testing"
import "encoding/binary"

func TestCollectionEmptyCollection(test *testing.T) {
	ctx := testctx("[collection.go]", test)

	basecollection := New()

	if !(basecollection.autoCollect.value) {
		test.Error("[collection.go]", "[autocollect]", "AutoCollect does not have true as default value.")
	}

	if !(basecollection.root.known) || !(basecollection.root.children.left.known) || !(basecollection.root.children.right.known) {
		test.Error("[collection.go]", "[known]", "New collection has unknown nodes.")
	}

	if !(basecollection.root.root()) {
		test.Error("[collection.go]", "[root]", "Collection root is not a root.")
	}

	if basecollection.root.leaf() {
		test.Error("[collection.go]", "[root]", "Collection root doesn't have children.")
	}

	if !(basecollection.root.children.left.placeholder()) || !(basecollection.root.children.right.placeholder()) {
		test.Error("[collection.go]", "[leaves]", "Collection leaves are not placeholder leaves.")
	}

	if len(basecollection.root.values) != 0 || len(basecollection.root.children.left.values) != 0 || len(basecollection.root.children.right.values) != 0 {
		test.Error("[collection.go]", "[values]", "Nodes of a collection without fields have values.")
	}

	ctx.verify.tree("[basecollection]", &basecollection)

	stake64 := Stake64{}
	stakecollection := New(stake64)

	if len(stakecollection.root.values) != 1 || len(stakecollection.root.children.left.values) != 1 || len(stakecollection.root.children.right.values) != 1 {
		test.Error("[collection.go]", "[values]", "Nodes of a stake collection don't have exactly one value.")
	}

	rootstake, rooterror := stake64.Decode(stakecollection.root.values[0])

	if rooterror != nil {
		test.Error("[collection.go]", "[stake]", "Malformed stake root value.")
	}

	leftstake, lefterror := stake64.Decode(stakecollection.root.children.left.values[0])

	if lefterror != nil {
		test.Error("[collection.go]", "[stake]", "Malformed stake left child value.")
	}

	rightstake, righterror := stake64.Decode(stakecollection.root.children.right.values[0])

	if righterror != nil {
		test.Error("[collection.go]", "[stake]", "Malformed stake right child value")
	}

	if rootstake.(uint64) != 0 || leftstake.(uint64) != 0 || rightstake.(uint64) != 0 {
		test.Error("[collection.go]", "[stake]", "Nodes of an empty stake collection don't have zero stake.")
	}

	ctx.verify.tree("[stakecollection]", &stakecollection)

	data := Data{}
	stakedatacollection := New(stake64, data)

	if len(stakedatacollection.root.values) != 2 || len(stakedatacollection.root.children.left.values) != 2 || len(stakedatacollection.root.children.right.values) != 2 {
		test.Error("[collection.go]", "[values]", "Nodes of a data and stake collection don't have exactly one value.")
	}

	if len(stakedatacollection.root.values[1]) != 0 || len(stakedatacollection.root.children.left.values[1]) != 0 || len(stakedatacollection.root.children.right.values[1]) != 0 {
		test.Error("[collection.go]", "[values]", "Nodes of a data and stake collection don't have empty data value.")
	}

	ctx.verify.tree("[stakedatacollection]", &stakedatacollection)
}

func TestCollectionEmptyVerifier(test *testing.T) {
	basecollection := New()
	baseverifier := NewVerifier()

	if !(baseverifier.autoCollect.value) {
		test.Error("[collection.go]", "[autocollect]", "AutoCollect does not have true as default value.")
	}

	if baseverifier.root.known {
		test.Error("[collection.go]", "[known]", "Empty verifier has known root.")
	}

	if (baseverifier.root.children.left != nil) || (baseverifier.root.children.right != nil) {
		test.Error("[collection.go]", "[root]", "Empty verifier root has children.")
	}

	if baseverifier.root.label != basecollection.root.label {
		test.Error("[collection.go]", "[label]", "Wrong verifier label.")
	}

	stake64 := Stake64{}

	stakecollection := New(stake64)
	stakeverifier := NewVerifier(stake64)

	if stakeverifier.root.known {
		test.Error("[collection.go]", "[known]", "Empty stake verifier has known root.")
	}

	if (stakeverifier.root.children.left != nil) || (stakeverifier.root.children.right != nil) {
		test.Error("[collection.go]", "[root]", "Empty stake verifier root has children.")
	}

	if stakeverifier.root.label != stakecollection.root.label {
		test.Error("[collection.go]", "[label]", "Wrong stake verifier label.")
	}

	data := Data{}

	stakedatacollection := New(stake64, data)
	stakedataverifier := NewVerifier(stake64, data)

	if stakedataverifier.root.known {
		test.Error("[collection.go]", "[known]", "Empty stake and data verifier has known root.")
	}

	if (stakedataverifier.root.children.left != nil) || (stakedataverifier.root.children.right != nil) {
		test.Error("[collection.go]", "[root]", "Empty stake and data verifier root has children.")
	}

	if stakedataverifier.root.label != stakedatacollection.root.label {
		test.Error("[collection.go]", "[label]", "Wrong stake and data verifier label.")
	}
}

func TestCollectionClone(test *testing.T) {
	ctx := testctx("[collection.go]", test)

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

	ctx.should_panic("[clone]", func() {
		collection.Begin()
		collection.Clone()
		collection.End()
	})
}

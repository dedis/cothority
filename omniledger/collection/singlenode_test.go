package collection

import "testing"

func TestSinglenodePlaceholder(test *testing.T) {
	basecollection := New()
	basenode := node{}
	basecollection.placeholder(&basenode)

	if !(basenode.placeholder()) {
		test.Error("[singlenode.go]", "[placeholder]", "placeholder() does not produce a placeholder node.")
	}

	if len(basenode.values) != 0 {
		test.Error("[singlenode.go]", "[values]", "Placeholder nodes of a collection without fields should not have values.")
	}

	stake64 := Stake64{}

	stakecollection := New(stake64)
	stakenode := node{}
	stakecollection.placeholder(&stakenode)

	if !(stakenode.placeholder()) {
		test.Error("[singlenode.go]", "[placeholder]", "placeholder() does not produce a placeholder node.")
	}

	if len(stakenode.values) != 1 {
		test.Error("[singlenode.go]", "[values]", "Placeholder nodes of a stake collection should have exactly one value.")
	}

	placeholderstake, _ := stake64.Decode(stakenode.values[0])
	if placeholderstake.(uint64) != 0 {
		test.Error("[singlenode.go]", "[value]", "Placeholder nodes of a stake collection should have zero stake.")
	}

	data := Data{}

	stakedatacollection := New(stake64, data)
	stakedatanode := node{}
	stakedatacollection.placeholder(&stakedatanode)

	if !(stakedatanode.placeholder()) {
		test.Error("[singlenode.go]", "[placeholder]", "placeholder() does not produce a placeholder node.")
	}

	if len(stakedatanode.values) != 2 {
		test.Error("[singlenode.go]", "[values]", "Placeholder nodes of a stake and data collection should have exactly two values.")
	}

	placeholderstake, _ = stake64.Decode(stakedatanode.values[0])
	if (placeholderstake.(uint64) != 0) || (len(stakedatanode.values[1]) != 0) {
		test.Error("[singlenode.go]", "[value]", "Placeholder nodes of a stake and data collection should have zero stake and empty data value.")
	}
}

func TestSinglenodeUpdate(test *testing.T) {
	ctx := testctx("[singlenode.go]", test)

	basecollection := New()

	error := basecollection.update(&node{})

	if error == nil {
		test.Error("[singlenode.go]", "[known]", "Update doesn't yield error on unknown node.")
	}

	basecollection.root.children.left.known = false
	error = basecollection.update(basecollection.root)

	if error == nil {
		test.Error("[singlenode.go]", "[known]", "Update doesn't yield error on node with unknown children.")
	}

	stake64 := Stake64{}
	stakecollection := New(stake64)

	stakecollection.root.children.left.values[0] = stake64.Encode(uint64(66))

	if stakecollection.update(stakecollection.root.children.left) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake leaf.")
	}

	if stakecollection.update(stakecollection.root) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake root.")
	}

	rootstake, _ := stake64.Decode(stakecollection.root.values[0])

	if rootstake.(uint64) != 66 {
		test.Error("[singlenode.go]", "[stake]", "Wrong value on stake root.")
	}

	stakecollection.root.children.right.values[0] = stake64.Encode(uint64(33))

	if stakecollection.update(stakecollection.root.children.right) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake leaf.")
	}

	if stakecollection.update(stakecollection.root) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake root.")
	}

	rootstake, _ = stake64.Decode(stakecollection.root.values[0])

	if rootstake.(uint64) != 99 {
		test.Error("[singlenode.go]", "[stake]", "Wrong value on stake root.")
	}

	ctx.verify.tree("[tree]", &stakecollection)

	stakecollection.root.children.left.values[0] = make([]byte, 5)

	if stakecollection.update(stakecollection.root) == nil {
		test.Error("[singlenode.go]", "[stake]", "Update() does not yield an error when updating a node with ill-formed children values.")
	}
}

package collection

import "testing"

func TestSingleNodeSetPlaceholder(test *testing.T) {
	baseCollection := New()
	baseNode := node{}
	err := baseCollection.setPlaceholder(&baseNode)
	if err != nil {
		test.Error("[singlenode.go]", "[setPlaceholder]", "SetPlaceholder returns an error:", err)
	}

	if !(baseNode.placeholder()) {
		test.Error("[singlenode.go]", "[placeholder]", "placeholder() does not produce a placeholder node.")
	}

	if len(baseNode.values) != 0 {
		test.Error("[singlenode.go]", "[values]", "Placeholder nodes of a collection without fields should not have values.")
	}

	stake64 := Stake64{}

	stakeCollection := New(stake64)
	stakeNode := node{}
	err = stakeCollection.setPlaceholder(&stakeNode)
	if err != nil {
		test.Error("[singlenode.go]", "[setPlaceholder]", "SetPlaceholder returns an error:", err)
	}

	if !(stakeNode.placeholder()) {
		test.Error("[singlenode.go]", "[placeholder]", "placeholder() does not produce a placeholder node.")
	}

	if len(stakeNode.values) != 1 {
		test.Error("[singlenode.go]", "[values]", "Placeholder nodes of a stake collection should have exactly one value.")
	}

	placeholderStake, _ := stake64.Decode(stakeNode.values[0])
	if placeholderStake.(uint64) != 0 {
		test.Error("[singlenode.go]", "[value]", "Placeholder nodes of a stake collection should have zero stake.")
	}

	data := Data{}

	stakeDataCollection := New(stake64, data)
	stakeDataNode := node{}
	err = stakeDataCollection.setPlaceholder(&stakeDataNode)
	if err != nil {
		test.Error("[singlenode.go]", "[setPlaceholder]", "setPlaceholder returns an error:", err)
	}

	if !(stakeDataNode.placeholder()) {
		test.Error("[singlenode.go]", "[placeholder]", "setPlaceholder() does not produce a placeholder node.")
	}

	if len(stakeDataNode.values) != 2 {
		test.Error("[singlenode.go]", "[values]", "Placeholder nodes of a stake and data collection should have exactly two values.")
	}

	placeholderStake, _ = stake64.Decode(stakeDataNode.values[0])
	if (placeholderStake.(uint64) != 0) || (len(stakeDataNode.values[1]) != 0) {
		test.Error("[singlenode.go]", "[value]", "Placeholder nodes of a stake and data collection should have zero stake and empty data value.")
	}
}

func TestSingleNodeUpdate(test *testing.T) {
	ctx := testCtx("[singlenode.go]", test)

	baseCollection := New()

	err := baseCollection.update(&node{})

	if err == nil {
		test.Error("[singlenode.go]", "[known]", "Update doesn't yield error on unknown node.")
	}

	baseCollection.root.children.left.known = false
	err = baseCollection.update(baseCollection.root)

	if err == nil {
		test.Error("[singlenode.go]", "[known]", "Update doesn't yield error on node with unknown children.")
	}

	stake64 := Stake64{}
	stakeCollection := New(stake64)

	stakeCollection.root.children.left.values[0] = stake64.Encode(uint64(66))

	if stakeCollection.update(stakeCollection.root.children.left) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake leaf.")
	}

	if stakeCollection.update(stakeCollection.root) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake root.")
	}

	rootStake, _ := stake64.Decode(stakeCollection.root.values[0])

	if rootStake.(uint64) != 66 {
		test.Error("[singlenode.go]", "[stake]", "Wrong value on stake root.")
	}

	stakeCollection.root.children.right.values[0] = stake64.Encode(uint64(33))

	if stakeCollection.update(stakeCollection.root.children.right) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake leaf.")
	}

	if stakeCollection.update(stakeCollection.root) != nil {
		test.Error("[singlenode.go]", "[stake]", "Update fails on stake root.")
	}

	rootStake, _ = stake64.Decode(stakeCollection.root.values[0])

	if rootStake.(uint64) != 99 {
		test.Error("[singlenode.go]", "[stake]", "Wrong value on stake root.")
	}

	ctx.verify.tree("[tree]", &stakeCollection)

	stakeCollection.root.children.left.values[0] = make([]byte, 5)

	if stakeCollection.update(stakeCollection.root) == nil {
		test.Error("[singlenode.go]", "[stake]", "Update() does not yield an error when updating a node with ill-formed children values.")
	}
}

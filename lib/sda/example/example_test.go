package example_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
)

// Tests a 2-node system
func TestNode2(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)

	h1, _ := sda.NewHostKey("localhost:2000")
	h2, _ := sda.NewHostKey("localhost:2000")
	defer h1.Close()
	defer h2.Close()

	list := sda.NewEntityList([]*network.Entity{h1.Entity, h2.Entity})
	tree, _ := list.GenerateBinaryTree()
	h1.AddEntityList(list)
	h1.AddTree(tree)

	h1.StartNewProtocolName("Example", tree.Id)
}

// Tests a 10-node system

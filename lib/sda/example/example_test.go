package example_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/sda/example"
	"testing"
	"time"
)

// Tests a 2-node system
func TestNode2(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocal()
	_, _, tree := local.GenTree(2, false, true)
	defer local.CloseAll()

	node, err := local.StartNewNodeName("Example", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := node.ProtocolInstance().(*example.ProtocolExample)

	select {
	case children := <-protocol.ChildCount:
		dbg.Lvl2("Instance 1 is done")
		if children != 2 {
			t.Fatal("Didn't get a child-cound of 2")
		}
	case <-time.After(time.Second):
		t.Fatal("Didn't finish in time")
	}
}

// Tests a 10-node system
func TestNode10(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocal()
	_, _, tree := local.GenTree(10, false, true)
	defer local.CloseAll()

	node, err := local.StartNewNodeName("Example", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := node.ProtocolInstance().(*example.ProtocolExample)

	select {
	case children := <-protocol.ChildCount:
		dbg.Lvl2("Instance 1 is done")
		if children != 2 {
			t.Fatal("Didn't get a child-cound of 2")
		}
	case <-time.After(time.Second):
		t.Fatal("Didn't finish in time")
	}
}

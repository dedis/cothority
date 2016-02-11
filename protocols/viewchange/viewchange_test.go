package view

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"testing"
	"time"
)

func TestViewChange(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)

	local := sda.NewLocalTest()

	_, el, tree := local.GenBigTree(4, 4, 1, true, true)
	defer local.CloseAll()

	// create the node
	sda.ProtocolRegisterName("ViewChange", func(n *sda.Node) (sda.ProtocolInstance, error) { return NewViewChange(n) })
	node, err := local.NewNodeEmptyName("ViewChange", tree)
	if err != nil {
		t.Fatal(err)
	}

	// create the new entitylist + new tree
	// just skip the last one
	list := el.List[:len(el.List)-1]
	el2 := sda.NewEntityList(list)
	tree2 := el2.GenerateBigNaryTree(1, 3)

	// create the instance protocol
	vc, err := NewViewChange(node)
	if err != nil {
		t.Fatal(err)
	}
	node.SetProtocolInstance(vc)
	// make the waiting for the agreement
	done := make(chan bool)
	fn := func() {
		done <- true
	}
	vc.RegisterOnDoneCallback(fn)

	// Propagate
	if err := vc.Propagate(tree2); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
		return
	case <-time.After(time.Second * 3):
		t.Fatal("Could not have the agreement view change protocol finished in time")
	}
}

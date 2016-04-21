package example_handlers_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/example/handlers"
)

// Tests a 2-node system
func TestNode(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true, true)
	//dbg.Lvl3(tree.Dump())
	defer local.CloseAll()

	pi, err := local.StartProtocol("ExampleHandlers", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := pi.(*example_handlers.ProtocolExampleHandlers)
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nbrNodes*2) * time.Millisecond
	select {
	case children := <-protocol.ChildCount:
		dbg.Lvl2("Instance 1 is done")
		if children != nbrNodes {
			t.Fatal("Didn't get a child-cound of", nbrNodes)
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

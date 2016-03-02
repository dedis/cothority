package example_channels_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/testutil"
	"github.com/dedis/cothority/protocols/example/channels"
	"testing"
	"time"
)

// Tests a 2-node system
func TestNode(t *testing.T) {
	defer testutil.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true, true)
	defer local.CloseAll()

	node, err := local.StartNewNodeName("ExampleChannels", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := node.ProtocolInstance().(*example_channels.ProtocolExampleChannels)

	select {
	case children := <-protocol.ChildCount:
		dbg.Lvl2("Instance 1 is done")
		if children != nbrNodes {
			t.Fatal("Didn't get a child-cound of", nbrNodes)
		}
	case <-time.After(time.Second * 10):
		// On a busy system this can take quite some time
		t.Fatal("Didn't finish in time")
	}
}

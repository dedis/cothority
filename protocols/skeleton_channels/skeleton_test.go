package skeleton_channels_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/skeleton_channels"
	"testing"
	"time"
)

// Tests a 2-node system
func TestNode(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true)
	defer local.CloseAll()

	node, err := local.StartNewNodeName("SkeletonChannels", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := node.ProtocolInstance().(*skeleton_channels.ProtocolSkeletonChannels)

	select {
	case children := <-protocol.ChildCount:
		dbg.Lvl2("Instance 1 is done")
		if children != nbrNodes {
			t.Fatal("Didn't get a child-cound of", nbrNodes)
		}
	case <-time.After(time.Second):
		t.Fatal("Didn't finish in time")
	}
}

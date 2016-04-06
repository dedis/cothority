package skipchain

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
	"time"
)

// Tests a 5-node system
func TestNode(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	nbrNodes := 5
	_, _, tree := local.GenTree(nbrNodes, false, true, true)
	//dbg.Lvl3(tree.Dump())
	defer local.CloseAll()

	node, err := local.StartNewNodeName("Skipchain", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := node.ProtocolInstance().(*ProtocolSkipchain)
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nbrNodes*2) * time.Millisecond
	select {
	case <-protocol.SetupDone:
		dbg.Lvl2("Setup is done")
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
	time.Sleep(time.Second)
}

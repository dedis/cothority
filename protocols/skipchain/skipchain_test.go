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
	nbrNodes := 2
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
	case <- protocol.SetupDone:
		dbg.Lvl3("Setup is done")
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
	lastblock:= []byte{0,1,2,3}

	err = protocol.SignNewBlock(tree.List())
	if err != nil {
		t.Fatal("Couldn't sign new block", err)
	}

	temp, err := protocol.LookUpBlock(lastblock)
	if err==nil {
		t.Fatal("didn't return Genesis")
	}

	
	lastblock = temp.ForwardLink[0].Hash
	temp, err = protocol.LookUpBlock(lastblock)
	if err!=nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
}

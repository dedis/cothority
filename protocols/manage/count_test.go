package manage_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/example/channels"
	"github.com/dedis/cothority/sda"
)

// Tests a 2-node system
func TestClose(t *testing.T) {
	local := sda.NewLocalTest()
	nbrNodes := 2
	_, _, tree := local.GenTree(nbrNodes, false, true, true)
	defer local.CloseAll()

	pi, err := local.CreateProtocol("ExampleChannels", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	go pi.Start()
	protocol := pi.(*channels.ProtocolExampleChannels)
	timeout := network.WaitRetry * time.Duration(network.MaxRetry*nbrNodes*2) * time.Millisecond
	select {
	case children := <-protocol.ChildCount:
		log.Lvl2("Instance 1 is done")
		if children != nbrNodes {
			t.Fatal("Didn't get a child-cound of", nbrNodes)
		}
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

package manage_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/manage"
)

// Tests a 2-node system
func TestBroadcast(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 3)
	for _, nbrNodes := range []int{3, 10, 14} {
		local := sda.NewLocalTest()
		_, _, tree := local.GenTree(nbrNodes, false, true, true)

		pi, err := local.CreateProtocol("Broadcast", tree)
		if err != nil {
			t.Fatal("Couldn't start protocol:", err)
		}
		protocol := pi.(*manage.Broadcast)
		done := make(chan bool)
		protocol.RegisterOnDone(func() {
			done <- true
		})
		protocol.Start()
		timeout := network.WaitRetry * time.Duration(network.MaxRetry*nbrNodes*2) * time.Millisecond
		select {
		case <-done:
			dbg.Lvl2("Done with connecting everybody")
		case <-time.After(timeout):
			t.Fatal("Didn't finish in time")
		}
		local.CloseAll()
	}
}

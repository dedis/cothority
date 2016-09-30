package manage_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/sda"
)

// Tests a 2-node system
func TestBroadcast(t *testing.T) {
	for _, nbrNodes := range []int{3, 10, 14} {
		local := sda.NewLocalTest()
		_, _, tree := local.GenTree(nbrNodes, false)

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
		timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
		select {
		case <-done:
			log.Lvl2("Done with connecting everybody")
		case <-time.After(timeout):
			t.Fatal("Didn't finish in time")
		}
		local.CloseAll()
		log.AfterTest(t)
	}
}

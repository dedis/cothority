package messaging_test

import (
	"testing"
	"time"

	"gopkg.in/dedis/cothority.v1/messaging"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// Tests a 2-node system
func TestBroadcast(t *testing.T) {
	for _, nbrNodes := range []int{3, 10, 14} {
		local := onet.NewLocalTest()
		_, _, tree := local.GenTree(nbrNodes, false)

		pi, err := local.CreateProtocol("Broadcast", tree)
		if err != nil {
			t.Fatal("Couldn't start protocol:", err)
		}
		protocol := pi.(*messaging.Broadcast)
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

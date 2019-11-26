package messaging

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

// Tests a 2-node system
func TestBroadcast(t *testing.T) {
	for _, nbrNodes := range []int{3, 10, 14} {
		local := onet.NewLocalTest(tSuite)
		_, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, false)

		pi, err := local.CreateProtocol(BroadcastName, tree)
		if err != nil {
			t.Fatal("Couldn't start protocol:", err)
		}
		protocol := pi.(*Broadcast)
		done := make(chan bool)
		protocol.RegisterOnDone(func() {
			done <- true
		})
		require.NoError(t, protocol.Start())
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

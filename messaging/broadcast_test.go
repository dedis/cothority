package messaging

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/ciphersuite"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
)

// Tests a 2-node system
func TestBroadcast(t *testing.T) {
	builder := onet.NewLocalBuilder(onet.NewDefaultBuilder())
	builder.SetSuite(&ciphersuite.UnsecureCipherSuite{})

	for _, nbrNodes := range []int{3, 10, 14} {
		local := onet.NewLocalTest(builder.Clone())
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

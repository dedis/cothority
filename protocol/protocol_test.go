package protocol

/*
The test-file should at the very least run the protocol for a varying number
of nodes. It is even better practice to test the different methods of the
protocol, as in Test Driven Development.
*/

import (
	"testing"

	"github.com/stretchr/testify/require"

	"time"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// Tests a 3, 5 and 13-node system.
func TestNode(t *testing.T) {
	runNodes(t, 5)
	//nodes := []int{3, 5, 13}
	//for _, nbrNodes := range nodes {
	//	runNodes(t, nbrNodes)
	//}
}

func runNodes(t *testing.T, nbrNodes int) {
	log.Lvl1("Running", nbrNodes, "nodes")
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	log.Lvl3(tree.Dump())

	pi, err := local.StartProtocol(Name, tree)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	protocol := pi.(*OnchainSecrets)
	timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	select {
	case <-protocol.done:
		log.Lvl2("root-node is done")
		require.NotNil(t, protocol.DKG)
		// Wait for other nodes
		time.Sleep(time.Second)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

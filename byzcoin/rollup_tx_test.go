package byzcoin

/*
The test-file should at the very least run the protocol for a varying number
of nodes. It is even better practice to test the different methods of the
protocol, as in Test Driven Development.
*/

import (
	"testing"
	"time"

	"github.com/dedis/cothority_template/protocol"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

//var tSuite = suites.MustFind("Ed25519")

// Tests a 2, 5 and 13-node system. It is good practice to test different
// sizes of trees to make sure your protocol is stable.
func TestNode(t *testing.T) {
	nodes := []int{2, 5, 13}
	for _, nbrNodes := range nodes {
		testNode(t, nbrNodes)
	}
}

func testNode(t *testing.T, nbrNodes int) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	_, _, tree := local.GenTree(nbrNodes, true)
	log.Lvl3(tree.Dump())

	pi, err := local.CreateProtocol("Template", tree)
	require.Nil(t, err)

	protocol := pi.(*protocol.TemplateProtocol)
	require.NoError(t, protocol.Start())

	timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	select {
	case children := <-protocol.ChildCount:
		log.Lvl2("Instance 1 is done")
		require.Equal(t, children, nbrNodes, "Didn't get a child-cound of", nbrNodes)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

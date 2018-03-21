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

	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// Tests a 3, 5 and 13-node system.
func TestSetupDKG(t *testing.T) {
	nodes := []int{3, 5, 10}
	for _, nbrNodes := range nodes {
		log.Lvlf1("Starting setupDKG with %d nodes", nbrNodes)
		setupDKG(t, nbrNodes)
	}
}

func setupDKG(t *testing.T, nbrNodes int) {
	log.Lvl1("Running", nbrNodes, "nodes")
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	_, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	log.Lvl3(tree.Dump())

	pi, err := local.CreateProtocol(NameDKG, tree)
	protocol := pi.(*SetupDKG)
	protocol.Wait = true
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	log.ErrFatal(pi.Start())
	timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	select {
	case <-protocol.SetupDone:
		log.Lvl2("root-node is Done")
		require.NotNil(t, protocol.DKG)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

package pedersen

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/kyber/v4/pairing/bn256"
	vss "go.dedis.ch/kyber/v4/share/vss/pedersen"
	"go.dedis.ch/kyber/v4/util/key"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestSetupDKG(t *testing.T) {
	nodes := []int{3, 5, 7}
	for _, nbrNodes := range nodes {
		log.Lvlf1("Starting setupDKG with %d nodes", nbrNodes)
		setupDKG(t, nbrNodes)
		setupCustomDKG(t, nbrNodes)
	}
}

func setupDKG(t *testing.T, nbrNodes int) {
	log.Lvl1("Running", nbrNodes, "nodes")
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	_, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	log.Lvl3(tree.Dump())

	pi, err := local.CreateProtocol(Name, tree)
	protocol := pi.(*Setup)
	protocol.Wait = true
	protocol.KeyPair = key.NewKeyPair(cothority.Suite)

	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	log.ErrFatal(pi.Start())
	timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	select {
	case <-protocol.Finished:
		log.Lvl2("root-node is Done")
		require.NotNil(t, protocol.DKG)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

func setupCustomDKG(t *testing.T, nbrNodes int) {
	log.Lvl1("Running", nbrNodes, "nodes")
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	srvs, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	log.Lvl3(tree.Dump())

	var name = "bn_dkg"
	var suite = bn256.NewSuite().G2().(vss.Suite)
	for _, srv := range srvs {
		_, err := srv.ProtocolRegister(name, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
			return CustomSetup(n, suite, key.NewKeyPair(suite))
		})
		require.NoError(t, err)
	}

	pi, err := local.CreateProtocol(name, tree)
	protocol := pi.(*Setup)
	protocol.Wait = true
	protocol.KeyPair = key.NewKeyPair(suite)

	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	log.ErrFatal(pi.Start())
	timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	select {
	case <-protocol.Finished:
		log.Lvl2("root-node is Done")
		require.NotNil(t, protocol.DKG)
	case <-time.After(timeout):
		t.Fatal("Didn't finish in time")
	}
}

package protocol

import (
	"testing"

	"errors"

	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// Used for tests
var testServiceID onet.ServiceID

const testServiceName = "ServiceOCS"

func init() {
	var err error
	testServiceID, err = onet.RegisterNewService(testServiceName, newService)
	log.ErrFatal(err)
}

// Tests a 3, 5 and 13-node system.
func TestOCS(t *testing.T) {
	nodes := []int{3, 5, 10}
	for _, nbrNodes := range nodes {
		log.Lvlf1("Starting setupDKG with %d nodes", nbrNodes)
		ocs(t, nbrNodes, nbrNodes-1, 32)
	}
}

func TestOCSKeyLengths(t *testing.T) {
	if testing.Short() {
		t.Skip("Testing all keylengths takes some time...")
	}
	for keylen := 1; keylen < 64; keylen++ {
		log.Lvl1("Testing keylen of", keylen)
		ocs(t, 3, 2, keylen)
	}
}

func ocs(t *testing.T, nbrNodes, threshold, keylen int) {
	log.Lvl1("Running", nbrNodes, "nodes")
	start := time.Now()
	local := onet.NewLocalTest()
	defer local.CloseAll()
	servers, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	log.Lvl3(tree.Dump())

	// 1 - setting up - in real life uses SetupDKG-protocol
	// Store the dkgs in the services
	dkgs, err := CreateDKGs(suite, nbrNodes, threshold)
	log.ErrFatal(err)
	services := local.GetServices(servers, testServiceID)
	for i := range services {
		services[i].(*testService).Shared, err = NewSharedSecret(dkgs[i])
		log.ErrFatal(err)
	}

	log.Lvl1("Setting up DKG without network:", time.Now().Sub(start))
	start = time.Now()

	// Get the collective public key
	dks, err := dkgs[0].DistKeyShare()
	log.ErrFatal(err)
	X := dks.Public()

	// 2 - writer - Encrypt a symmetric key and publish U, Cs
	key := random.Bytes(keylen, random.Stream)
	U, Cs := EncodeKey(suite, X, key)

	// 3 - reader - Makes a request to U by giving his public key Xc
	// xc is the client's private/publick key pair
	xc := config.NewKeyPair(network.Suite)

	log.Lvl1("Encrypting key (no interaction):", time.Now().Sub(start))
	start = time.Now()

	// 4 - service - starts the protocol -
	// as every node needs to have its own DKG, we
	// use a service to give the corresponding DKGs to the nodes.
	pi, err := services[0].(*testService).startOCS(tree)
	log.ErrFatal(err)
	protocol := pi.(*OCS)
	protocol.U = U
	protocol.Xc = xc.Public
	// timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	log.ErrFatal(protocol.Start())
	select {
	case <-protocol.Done:
		log.Lvl2("root-node is done")
		require.NotNil(t, protocol.Uis)
		// Wait for other nodes
	case <-time.After(time.Hour):
		t.Fatal("Didn't finish in time")
	}

	log.Lvl1("Running re-encryption:", time.Now().Sub(start))
	start = time.Now()

	// 5 - service - Lagrange interpolate the Uis - the reader will only
	// get XhatEnc
	XhatEnc, err := share.RecoverCommit(suite, protocol.Uis, threshold, nbrNodes)
	log.ErrFatal(err)

	// 6 - reader - gets the resulting symmetric key, encrypted under Xc
	keyHat, err := DecodeKey(suite, X, Cs, XhatEnc, xc.Secret)
	log.ErrFatal(err)

	log.Lvl1("Decrypting the key:", time.Now().Sub(start))

	require.Equal(t, key, keyHat)
}

// testService allows setting the dkg-field of the protocol.
type testService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	// Has to be initialised by the test
	Shared *SharedSecret
}

// Creates a service-protocol and returns the ProtocolInstance.
func (s *testService) startOCS(t *onet.Tree) (onet.ProtocolInstance, error) {
	pi, err := s.CreateProtocol(NameOCS, t)
	pi.(*OCS).Shared = s.Shared
	return pi, err
}

// Store the dkg in the protocol
func (s *testService) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	switch tn.ProtocolName() {
	case NameOCS:
		pi, err := NewOCS(tn)
		if err != nil {
			return nil, err
		}
		ocs := pi.(*OCS)
		ocs.Shared = s.Shared
		return ocs, nil
	default:
		return nil, errors.New("unknown protocol for this service")
	}
}

// starts a new service. No function needed.
func newService(c *onet.Context) onet.Service {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s
}

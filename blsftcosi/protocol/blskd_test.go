package protocol

import (
	"testing"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// Used for tests
var testKDServiceID onet.ServiceID

const testKDServiceName = "ServiceBlsFtKD"

func init() {
	var err error
	testKDServiceID, err = onet.RegisterNewService(testKDServiceName, newKDService)
	log.ErrFatal(err)
	// Register Protocols. Should not be required. TODO
	onet.GlobalProtocolRegister(DefaultKDProtocolName, NewBlsKeyDist)
}

// Test various trees configuration
func TestKDProtocol(t *testing.T) {
	log.SetDebugVisible(3)
	nodes := []int{1, 2, 5, 13, 24}
	subtrees := []int{1, 2, 5, 9}

	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			if nSubtrees >= nNodes && nSubtrees > 1 {
				continue
			}
			log.Lvl2("test asking for", nNodes, "nodes and", nSubtrees, "subtrees")

			local := onet.NewLocalTest(testSuite)
			servers, _, tree := local.GenTree(nNodes, false)

			services := local.GetServices(servers, testKDServiceID)

			rootService := services[0].(*testKDService)
			pi, err := rootService.CreateProtocol(DefaultKDProtocolName, tree)
			if err != nil {
				local.CloseAll()
				t.Fatal("Error in creation of protocol:", err)
			}
			blskeydist := pi.(*BlsKeyDist)
			blskeydist.PairingPublic = rootService.public
			blskeydist.Timeout = defaultTimeout

			err = blskeydist.Start()
			if err != nil {
				log.Lvl3("Error while running the protocol", err)
				local.CloseAll()
				t.Fatal(err)
			}

			// get and verify keys
			var publicKeys []kyber.Point
			select {
			case publicKeys = <-blskeydist.PairingPublics:
				log.Lvl3("Instance is done")
			case <-time.After(defaultTimeout * 2):
				// wait a bit longer than the protocol timeout
				log.Lvl3("Didnt receive commitment in time")
				t.Fatal("didnt get commitment in time")
			}

			for i, service := range services {
				tService := service.(*testKDService)
				if i >= len(publicKeys) || !publicKeys[i].Equal(tService.public) {
					t.Fatal("didn't get valid public key for node", i, publicKeys[i], tService.public)
				}
			}
			log.Lvl3("Public Keys correctly verified!")
			local.CloseAll()
		}
	}
}

// testKDService initializes the pairing keys of the protocol.
type testKDService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	// Has to be initialized by the test
	private kyber.Scalar
	public  kyber.Point
}

// Starts a new service. No function needed.
func newKDService(c *onet.Context) (onet.Service, error) {
	s := &testKDService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	s.private, s.public = bls.NewKeyPair(pairingSuite, random.New())
	return s, nil
}

// Store the public and private keys in the protocol
func (s *testKDService) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Cosi Service received New Protocol event")

	pi, err := NewBlsKeyDist(tn)
	if err != nil {
		return nil, err
	}
	blskeydist := pi.(*BlsKeyDist)
	blskeydist.PairingPublic = s.public
	blskeydist.Timeout = defaultTimeout
	return blskeydist, nil
}

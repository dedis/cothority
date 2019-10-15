package bdnproto

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
)

var testSuite = pairing.NewSuiteBn256()

const testServiceName = "TestServiceBdnCosi"

var testServiceID onet.ServiceID

func init() {
	GlobalRegisterBdnProtocols()

	id, err := onet.RegisterNewServiceWithSuite(testServiceName, testSuite, newService)
	if err != nil {
		log.Fatal(err)
	}

	testServiceID = id
}

func TestBdnProto_SimpleCase(t *testing.T) {
	err := runProtocol(5, 1, 5)
	require.NoError(t, err)

	if !testing.Short() {
		err = runProtocol(10, 5, 10)
		require.NoError(t, err)

		err = runProtocol(20, 5, 15)
		require.NoError(t, err)
	}
}

func runProtocol(nbrNodes, nbrSubTrees, threshold int) error {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers, roster, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceID)

	rootService := services[0].(*testService)
	pi, err := rootService.CreateProtocol(BdnProtocolName, tree)
	if err != nil {
		return err
	}

	cosiProtocol := pi.(*protocol.BlsCosi)
	cosiProtocol.CreateProtocol = rootService.CreateProtocol
	cosiProtocol.Msg = []byte{0xFF}
	cosiProtocol.Timeout = 10 * time.Second
	cosiProtocol.Threshold = threshold
	if nbrSubTrees > 0 {
		err = cosiProtocol.SetNbrSubTree(nbrSubTrees)
		if err != nil {
			return err
		}
	}

	err = cosiProtocol.Start()
	if err != nil {
		return err
	}

	select {
	case sig := <-cosiProtocol.FinalSignature:
		pubs := roster.ServicePublics(testServiceName)
		return BdnSignature(sig).Verify(testSuite, cosiProtocol.Msg, pubs)
	case <-time.After(2 * time.Second):
	}

	return errors.New("timeout")
}

// testService allows setting the pairing keys of the protocol.
type testService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
}

// Starts a new service. No function needed.
func newService(c *onet.Context) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s, nil
}

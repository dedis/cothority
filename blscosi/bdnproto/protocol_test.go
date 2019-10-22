package bdnproto

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v4/blscosi/protocol"
	"go.dedis.ch/cothority/v4/cosuite"
	"go.dedis.ch/onet/ciphersuite"
	"go.dedis.ch/onet/v4"
	"golang.org/x/xerrors"
)

var testSuite = cosuite.NewBdnSuite()

const testServiceName = "TestServiceBdnCosi"

func makeTestBuilder() onet.Builder {
	builder := onet.NewLocalBuilder(onet.NewDefaultBuilder())
	builder.SetSuite(testSuite)
	builder.SetService(testServiceName, nil, newService)
	return builder
}

func init() {
	GlobalRegisterBdnProtocols()
}

func TestBdnProto_SimpleCase(t *testing.T) {
	err := runProtocol(5, 1, 5)
	require.NoError(t, err)
}

func TestBdnProto_LongTest(t *testing.T) {
	if !testing.Short() {
		err := runProtocol(10, 5, 10)
		require.NoError(t, err)

		err = runProtocol(20, 5, 15)
		require.NoError(t, err)
	}
}

func runProtocol(nbrNodes, nbrSubTrees, threshold int) error {
	local := onet.NewLocalTest(makeTestBuilder())
	defer local.CloseAll()
	servers, _, tree := local.GenTree(nbrNodes, false)

	services := local.GetServices(servers, testServiceName)

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
		if sig == nil {
			return xerrors.New("missing signature")
		}

		pubs := cosiProtocol.PublicKeys()

		pk, err := testSuite.AggregatePublicKeys(pubs, sig)
		if err != nil {
			return err
		}

		return testSuite.Verify(pk, sig, cosiProtocol.Msg)
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
func newService(c *onet.Context, suite ciphersuite.CipherSuite) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s, nil
}

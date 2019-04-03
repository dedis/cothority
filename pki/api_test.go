package pki

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
)

func init() {
	onet.RegisterNewService(corruptedServiceName, newCorruptedService)
}

func TestAPI_GetProof(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	server := local.GenServers(1)[0]

	client := NewClient()
	proofs, err := client.GetProof(server.ServerIdentity)
	require.NoError(t, err)
	require.Equal(t, 2, len(proofs))
}

func TestAPI_CorruptedGetProof(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	server := local.GenServers(1)[0]
	service := server.Service(corruptedServiceName).(*corruptedService)

	client := newCorruptedClient()

	service.response = &ResponsePkProof{Proofs: PkProofs{}}
	_, err := client.GetProof(server.ServerIdentity)
	require.Error(t, err)
	require.Equal(t, "got a wrong proof for service testServiceA", err.Error())

	service.err = errors.New("test")
	_, err = client.GetProof(server.ServerIdentity)
	require.Error(t, err)
	require.Equal(t, "request failed with: websocket: close 4000: test", err.Error())
}

func newCorruptedClient() *Client {
	return &Client{
		Client: onet.NewClient(cothority.Suite, corruptedServiceName),
	}
}

const corruptedServiceName = "pki-service-corrupted"

type corruptedService struct {
	*onet.ServiceProcessor

	err      error
	response *ResponsePkProof
}

func newCorruptedService(c *onet.Context) (onet.Service, error) {
	service := &corruptedService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}

	err := service.RegisterHandlers(service.RequestProof)

	return service, err
}

func (cs *corruptedService) RequestProof(req *RequestPkProof) (*ResponsePkProof, error) {
	return cs.response, cs.err
}

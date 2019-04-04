package pki

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
)

func init() {
	onet.RegisterNewServiceWithSuite("testServiceA", bn256Suite, newPKIService)
	onet.RegisterNewServiceWithSuite("testServiceB", cothority.Suite, newPKIService)
}

func TestService_GetProof(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	servers := local.GenServers(2)

	service := servers[0].Service(ServiceName).(*Service)
	proofs, err := service.GetProof(servers[1].ServerIdentity)
	require.NoError(t, err)
	require.Equal(t, 2, len(proofs))

	servers[1].Pause()
	proofs, err = service.GetProof(servers[1].ServerIdentity)
	require.NoError(t, err)
	require.Equal(t, 2, len(proofs))
}

func TestService_VerifyRoster(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	servers, ro, _ := local.GenTree(4, false)
	service := servers[0].Service(ServiceName).(*Service)

	err := service.VerifyRoster(ro)
	require.NoError(t, err)

	ro.List[1].ServiceIdentities[0].Public = bn256Suite.Point().Base()
	err = service.VerifyRoster(ro)
	require.Error(t, err)
	require.Contains(t, err.Error(), "couldn't verify ")
}

func TestService_RequestProof(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	server := local.GenServers(1)[0]
	service := server.Service(ServiceName).(*Service)

	rep, err := service.RequestProof(&RequestPkProof{})
	require.NoError(t, err)
	require.Equal(t, 2, len(rep.Proofs))

	require.Nil(t, rep.Proofs.Verify(&server.ServerIdentity.ServiceIdentities[0]))
	require.Nil(t, rep.Proofs.Verify(&server.ServerIdentity.ServiceIdentities[1]))

	server.ServerIdentity.ServiceIdentities[0].Suite = "unknown"
	_, err = service.RequestProof(&RequestPkProof{})
	require.Error(t, err)
	require.Equal(t, "unknown suite for the service key pair", err.Error())
}

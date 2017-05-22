package service

import (
	"testing"

	"github.com/dedis/cothority/identity"
	"github.com/dedis/cothority/skipchain"
	_ "github.com/dedis/cothority/skipchain/service"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateIdentity(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	nodes, r, _ := local.GenTree(2, true)
	defer local.CloseAll()
	service := local.GetServices(nodes, identityService)[0].(*Service)

	root, cerr := skipchain.NewClient().CreateGenesis(r, 1, 1, identity.VerificationIdentity, nil, nil)
	log.ErrFatal(cerr)

	cir, cerr := service.CreateIdentity(&identity.CreateIdentity{
		Roster: root.Roster,
		Config: &identity.Config{}})
	log.ErrFatal(cerr)
	require.NotNil(t, cir.Genesis)
	require.Equal(t, 1, len(service.StorageMap.Identities))
	stor := service.StorageMap.Identities[string(cir.Genesis.Hash)]
	require.Equal(t, &identity.Config{}, stor.Latest)
}

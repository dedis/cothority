package identity

import (
	"testing"

	"github.com/dedis/cothority/skipchain"
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

	root, cerr := skipchain.NewClient().CreateGenesis(r, 1, 1, verificationIdentity, nil, nil)
	log.ErrFatal(cerr)

	cir, cerr := service.CreateIdentity(&CreateIdentity{&Config{}, root})
	log.ErrFatal(cerr)
	require.NotNil(t, cir.Data)
	require.Equal(t, 1, len(service.StorageMap.Identities))
	stor := service.StorageMap.Identities[string(cir.Data.Hash)]
	require.Equal(t, &Config{}, stor.Latest)
}

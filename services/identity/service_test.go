package identity

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	//log.Info("Skipping because of bftcosi and skipchain - #482")
	sda.RegisterNewService(ServiceName, func(c *sda.Context, path string) sda.Service {
		s := newIdentityService(c, path).(*Service)
		s.skipchain = skipchain.NewTestClient()
		return s
	})
	log.MainTest(m)
}

func TestService_AddIdentity(t *testing.T) {
	log.TestOutput(true, 3)
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, el, s := local.MakeHELS(5, identityService)
	service := s.(*Service)

	keypair := config.NewKeyPair(network.Suite)
	il := NewConfig(50, keypair.Public, "one")
	msg, err := service.AddIdentity(nil, &AddIdentity{il, el})
	log.ErrFatal(err)
	air := msg.(*AddIdentityReply)

	data := air.Data
	id, ok := service.identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

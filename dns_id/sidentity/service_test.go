package sidentity

/*
import (
	"testing"

	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/onet"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateIdentity2(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	_, el, s := local.MakeHELS(5, identityService)
	service := s.(*Service)

	keypair := config.NewKeyPair(network.Suite)
	il := NewConfig(50, keypair.Public, "one")
	msg, err := service.CreateIdentity(nil, &CreateIdentity{il, el})
	log.ErrFatal(err)
	air := msg.(*CreateIdentityReply)

	data := air.Data
	id, ok := service.Identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}
*/

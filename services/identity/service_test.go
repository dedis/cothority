package identity

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/config"
	"github.com/stretchr/testify/assert"
)

func TestService_AddIdentity(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, el, s := local.MakeHELS(5, identityService)
	service := s.(*Service)

	keypair := config.NewKeyPair(network.Suite)
	il := NewAccountList(50, keypair.Public, "one", "public1")
	msg, err := service.AddIdentity(nil, &AddIdentity{il, el})
	log.ErrFatal(err)
	air := msg.(*AddIdentityReply)

	data := air.Data
	id, ok := service.identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

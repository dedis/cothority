package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/anon"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateIdentity2(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	_, el, s := local.MakeHELS(5, identityService)
	service := s.(*Service)

	kp := config.NewKeyPair(network.Suite)
	kp2 := config.NewKeyPair(network.Suite)
	set := anon.Set([]abstract.Point{kp.Public, kp2.Public})
	service.auth.sets = append(service.auth.sets, set)

	il := NewData(50, kp.Public, "one")
	ci := &CreateIdentity{}
	ci.Data = il
	ci.Roster = el
	ci.Nonce = random.Bytes(nonceSize, random.Stream)
	service.auth.nonces[string(ci.Nonce)] = struct{}{}
	ctx := []byte(ServiceName + service.ServerIdentity().String())

	ci.Sig = anon.Sign(network.Suite, random.Stream, ci.Nonce,
		set, ctx, 0, kp.Secret)
	msg, cerr := service.CreateIdentity(ci)
	log.ErrFatal(cerr)
	air := msg.(*CreateIdentityReply)

	data := air.Data
	id, ok := service.Identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/config"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/crypto"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
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
	set := anon.Set([]kyber.Point{kp.Public, kp2.Public})
	service.auth.sets = append(service.auth.sets, set)

	il := NewData(50, kp.Public, "one")
	ci := &CreateIdentity{}
	ci.Type = PoPAuth
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

func TestService_CreateIdentity3(t *testing.T) {
	local := onet.NewTCPTest()
	defer local.CloseAll()
	_, el, s := local.MakeHELS(5, identityService)
	service := s.(*Service)

	kp := config.NewKeyPair(network.Suite)
	service.auth.keys = append(service.auth.keys, kp.Public)

	il := NewData(50, kp.Public, "one")
	ci := &CreateIdentity{}
	ci.Type = PublicAuth
	ci.Data = il
	ci.Roster = el
	ci.Public = kp.Public
	ci.Nonce = random.Bytes(nonceSize, random.Stream)
	service.auth.nonces[string(ci.Nonce)] = struct{}{}
	var err error
	ci.SchnSig, err = crypto.SignSchnorr(network.Suite, kp.Secret, ci.Nonce)
	log.ErrFatal(err)
	msg, cerr := service.CreateIdentity(ci)
	log.ErrFatal(cerr)
	air := msg.(*CreateIdentityReply)

	data := air.Data
	id, ok := service.Identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

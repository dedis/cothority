package identity

import (
	"testing"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_CreateIdentity2(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	defer local.CloseAll()
	_, ro, s := local.MakeSRS(tSuite, 5, identityService)
	service := s.(*Service)

	kp := key.NewKeyPair(tSuite)
	kp2 := key.NewKeyPair(tSuite)
	set := anon.Set([]kyber.Point{kp.Public, kp2.Public})
	service.Storage.Auth.sets = append(service.Storage.Auth.sets, set)

	da := NewData(ro, 50, kp.Public, "one")
	ci := &CreateIdentity{}
	ci.Type = PoPAuth
	ci.Data = da
	ci.Nonce = make([]byte, nonceSize)
	random.Bytes(ci.Nonce, random.New())
	service.Storage.Auth.nonces[string(ci.Nonce)] = struct{}{}
	ctx := []byte(ServiceName + service.ServerIdentity().String())

	ci.Sig = anon.Sign(tSuite, ci.Nonce,
		set, ctx, 0, kp.Private)
	air, err := service.CreateIdentity(ci)
	log.ErrFatal(err)

	data := air.Genesis
	id, ok := service.Storage.Identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

func TestService_CreateIdentity3(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	defer local.CloseAll()
	_, ro, s := local.MakeSRS(tSuite, 5, identityService)
	service := s.(*Service)

	kp := key.NewKeyPair(tSuite)
	service.Storage.Auth.keys = append(service.Storage.Auth.keys, kp.Public)

	da := NewData(ro, 50, kp.Public, "one")
	ci := &CreateIdentity{}
	ci.Type = PublicAuth
	ci.Data = da
	ci.Nonce = make([]byte, nonceSize)
	random.Bytes(ci.Nonce, tSuite.RandomStream())
	service.Storage.Auth.nonces[string(ci.Nonce)] = struct{}{}
	var err error
	ssig, err := schnorr.Sign(tSuite, kp.Private, ci.Nonce)
	ci.SchnSig = &ssig
	log.ErrFatal(err)
	air, err := service.CreateIdentity(ci)
	log.ErrFatal(err)

	data := air.Genesis
	id, ok := service.Storage.Identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

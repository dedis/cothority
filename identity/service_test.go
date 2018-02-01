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
	_, el, s := local.MakeHELS(5, identityService, tSuite)
	service := s.(*Service)

	kp := key.NewKeyPair(tSuite)
	kp2 := key.NewKeyPair(tSuite)
	set := anon.Set([]kyber.Point{kp.Public, kp2.Public})
	service.auth.sets = append(service.auth.sets, set)

	il := NewData(50, kp.Public, "one")
	ci := &CreateIdentity{}
	ci.Type = PoPAuth
	ci.Data = il
	ci.Roster = el
	ci.Nonce = make([]byte, nonceSize)
	random.Bytes(ci.Nonce, random.New())
	service.auth.nonces[string(ci.Nonce)] = struct{}{}
	ctx := []byte(ServiceName + service.ServerIdentity().String())

	ci.Sig = anon.Sign(tSuite, ci.Nonce,
		set, ctx, 0, kp.Private)
	air, err := service.CreateIdentity(ci)
	log.ErrFatal(err)

	data := air.Data
	id, ok := service.Identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

func TestService_CreateIdentity3(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	defer local.CloseAll()
	_, el, s := local.MakeHELS(5, identityService, tSuite)
	service := s.(*Service)

	kp := key.NewKeyPair(tSuite)
	service.auth.keys = append(service.auth.keys, kp.Public)

	il := NewData(50, kp.Public, "one")
	ci := &CreateIdentity{}
	ci.Type = PublicAuth
	ci.Data = il
	ci.Roster = el
	ci.Public = kp.Public
	ci.Nonce = make([]byte, nonceSize)
	random.Bytes(ci.Nonce, tSuite.RandomStream())
	service.auth.nonces[string(ci.Nonce)] = struct{}{}
	var err error
	ssig, err := schnorr.Sign(tSuite, kp.Private, ci.Nonce)
	ci.SchnSig = &ssig
	log.ErrFatal(err)
	air, err := service.CreateIdentity(ci)
	log.ErrFatal(err)

	data := air.Data
	id, ok := service.Identities[string(data.Hash)]
	assert.True(t, ok)
	assert.NotNil(t, id)
}

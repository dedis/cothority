package swupdate

import (
	"testing"

	"os"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func init() {
	initGlobals(3)
}

func TestMain(m *testing.M) {
	os.RemoveAll("config")
	log.MainTest(m)
}

func TestServiceSaveLoad(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, _, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)

	service.save()

	s2 := &Service{path: service.path}
	log.ErrFatal(s2.tryLoad())
}

func TestService_CreatePackage(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	// This should fail as the signatures are wrong
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{
			Roster:  roster,
			Release: &Release{policy1, sigs2},
			Base:    2,
			Height:  10,
		})
	assert.NotNil(t, err, "Accepted wrong signatures")
	cpr, err = service.CreatePackage(nil,
		&CreatePackage{roster, release1, 2, 10})
	log.ErrFatal(err)

	sc := cpr.(*CreatePackageRet).SwupChain
	assert.NotNil(t, sc.Data)
	assert.NotNil(t, sc.Timestamp)
	policy := sc.Release.Policy
	assert.Equal(t, *policy1, *policy)
	assert.Equal(t, *policy1, *service.StorageMap.Storage[policy.Name].Release.Policy)
}

func TestService_UpdatePackage(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{roster, release1, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain
	upr, err := service.UpdatePackage(nil,
		&UpdatePackage{
			SwupChain: sc,
			Release:   &Release{policy2, sigs1},
		})
	assert.NotNil(t, err, "Updating packages with wrong signature should fail")
	upr, err = service.UpdatePackage(nil,
		&UpdatePackage{
			SwupChain: sc,
			Release:   release2,
		})
	log.ErrFatal(err)
	sc = upr.(*SwupChain)
	assert.NotNil(t, sc)
	assert.Equal(t, *policy2, *sc.Release.Policy)
}

var keys []*PGP
var keysPublic []string

var policy1 *Policy
var policy2 *Policy

var sigs1 []string
var sigs2 []string

var release1 *Release
var release2 *Release

func initGlobals(nbrKeys int) {
	for i := 0; i < nbrKeys; i++ {
		keys = append(keys, NewPGP())
		keysPublic = append(keysPublic, keys[i].ArmorPublic())
	}

	policy1 = &Policy{
		Name:       "test",
		Version:    "1.2",
		Source:     "https://github.com/dedis/cothority",
		Threshold:  3,
		Keys:       keysPublic,
		BinaryHash: "0",
	}
	policy2 = &Policy{
		Name:       "test",
		Version:    "1.3",
		Source:     "https://github.com/dedis/cothority",
		Threshold:  3,
		Keys:       keysPublic,
		BinaryHash: "0",
	}

	p1, err := network.MarshalRegisteredType(policy1)
	log.ErrFatal(err)
	p2, err := network.MarshalRegisteredType(policy2)
	log.ErrFatal(err)
	for _, k := range keys {
		s1, err := k.Sign(p1)
		log.ErrFatal(err)
		s2, err := k.Sign(p2)
		log.ErrFatal(err)

		sigs1 = append(sigs1, s1)
		sigs2 = append(sigs2, s2)
	}

	release1 = &Release{policy1, sigs1}
	release2 = &Release{policy2, sigs2}
}

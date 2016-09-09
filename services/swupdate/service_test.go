package swupdate

import (
	"testing"

	"os"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

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

var policyLines1 = `
name = "test"
version = "1.2"
source = "https://github.com/dedis/cothority"
threshold = 3
keys = [ "one", "two" ]
binaryHash = "1234"
`
var policyLines2 = `
name = "test"
version = "1.3"
source = "https://github.com/dedis/cothority"
threshold = 3
keys = [ "one", "two" ]
binaryHash = "1235"
`

var sigs1 = NewSignatures(`
signatures = [ "sig1", "sig2" ]
`)
var sigs2 = NewSignatures(`
signatures = [ "sig3", "sig4" ]
`)

func TestService_CreatePackage(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	policy, err := NewPolicy(policyLines1)
	log.ErrFatal(err)
	// This should fail as the signatures are wrong
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{roster, policy, sigs2, 2, 10})
	assert.NotNil(t, err, "Accepted wrong signatures")
	cpr, err = service.CreatePackage(nil,
		&CreatePackage{roster, policy, sigs1, 2, 10})
	log.ErrFatal(err)

	sc := cpr.(*CreatePackageRet).SwupChain
	assert.NotNil(t, sc.Data)
	assert.NotNil(t, sc.Timestamp)
	assert.Equal(t, *policy, *sc.Policy)
	assert.Equal(t, *policy, *service.StorageMap.Storage[sc.Policy.Name].Policy)
}

func TestService_UpdatePackage(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	policy, err := NewPolicy(policyLines1)
	log.ErrFatal(err)
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{roster, policy, sigs1, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain
	policy2, err := NewPolicy(policyLines2)
	log.ErrFatal(err)
	upr, err := service.UpdatePackage(nil,
		&UpdatePackage{
			SwupChain:  sc,
			Policy:     policy2,
			Signatures: sigs1,
		})
	assert.NotNil(t, err, "Updating packages with wrong signature should fail")
	upr, err = service.UpdatePackage(nil,
		&UpdatePackage{
			SwupChain:  sc,
			Policy:     policy2,
			Signatures: sigs2,
		})
	log.ErrFatal(err)
	sc = upr.(*SwupChain)
	assert.NotNil(t, sc)
	assert.Equal(t, *policy2, *sc.Policy)
}

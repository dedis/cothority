package swupdate

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceSaveLoad(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, _, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)

	service.StorageMap.Name = "test"
	service.save()

	s2 := &Service{path: service.path}
	log.ErrFatal(s2.tryLoad())
	assert.Equal(t, "test", s2.StorageMap.Name)
}

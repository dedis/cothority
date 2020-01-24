package evoting_test

import (
	"testing"

	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"

	"github.com/stretchr/testify/assert"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/evoting"
	_ "go.dedis.ch/cothority/v3/evoting/service"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestPing(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenTree(3, true)

	c := evoting.NewClient()
	r, _ := c.Ping(roster, 0)
	assert.Equal(t, uint32(1), r.Nonce)
}

package evoting_test

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	_ "github.com/dedis/cothority/evoting/service"
)

func TestPing(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenTree(3, true)

	c := evoting.NewClient()
	r, _ := c.Ping(roster, 0)
	assert.Equal(t, uint32(1), r.Nonce)
}

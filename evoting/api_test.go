package evoting

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
)

func TestPing(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	// 	_, roster, _ := local.GenTree(3, true)

	// 	c := NewClient()
	// 	r, _ := c.Ping(roster, 0)
	// 	assert.Equal(t, 1, r.Nonce)
}

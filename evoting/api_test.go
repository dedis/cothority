package evoting

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/dedis/cothority/evoting/lib"
)

func TestPing(t *testing.T) {
	local := onet.NewTCPTest(lib.Suite)
	defer local.CloseAll()

	// 	_, roster, _ := local.GenTree(3, true)

	// 	c := NewClient()
	// 	r, _ := c.Ping(roster, 0)
	// 	assert.Equal(t, 1, r.Nonce)
}

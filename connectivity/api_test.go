package connectivity_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/connectivity"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestCheck(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenTree(3, true)

	c := connectivity.NewClient()
	r, err := c.Check(roster.Get(0), roster)
	assert.NoError(t, err)

	for node, state := range r.Status {
		assert.False(t, state.Down, fmt.Sprintf("%s is down", node))
	}

	time.Sleep(5 * time.Second)
}

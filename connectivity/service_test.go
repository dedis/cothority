package connectivity

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
)

func TestService(t *testing.T) {
	os.Setenv("CONNECTIVITY_TTL", "1")
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenTree(7, true)

	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	cr, err := s0.Check(&CheckRequest{Roster: roster})
	assert.NoError(t, err)

	for node, state := range cr.Status {
		assert.False(t, state.Down, fmt.Sprintf("%s is down", node))
	}

	time.Sleep(2 * time.Second)

	err = nodes[1].Stop()
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	cr, err = s0.Check(&CheckRequest{Roster: roster})
	assert.NoError(t, err)

	assert.True(t, cr.Status[nodes[1].Address().String()].Down)
}

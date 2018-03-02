package lib

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority"
)

func TestChain(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	_, err := chain(roster, []byte{})
	assert.NotNil(t, err)

	election := &Election{Roster: roster, Stage: Running}
	_ = election.GenChain(10)

	chain, _ := chain(roster, election.ID)
	assert.NotNil(t, chain)
}

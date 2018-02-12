package lib

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"
)

func TestChain(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	_, err := chain(roster, []byte{})
	assert.NotNil(t, err)

	election := &Election{Roster: roster, Stage: RUNNING}
	_ = election.GenChain(10)

	chain, _ := chain(roster, election.ID)
	assert.NotNil(t, chain)
}

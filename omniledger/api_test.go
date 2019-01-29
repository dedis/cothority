package omniledger

import (
	"github.com/dedis/onet/network"
	//"fmt"
	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClient_getRosterChangesCount(t *testing.T) {
	copyRoster := func(roster *onet.Roster) *onet.Roster {
		cpy := &onet.Roster{}
		cpy.ID = roster.ID

		cpy.List = make([]*network.ServerIdentity, 0)
		for _, server := range roster.List {
			cpy.List = append(cpy.List, server)
		}

		cpy.Aggregate = roster.Aggregate

		return cpy
	}

	l := onet.NewLocalTest(cothority.Suite)
	defer l.CloseAll()

	i := 3
	j := 3

	_, roster1, _ := l.GenTree(i, true)
	_, roster2, _ := l.GenTree(j, true)

	// Check the number of changes is the same regardless of parameter order
	assert.True(t, getRosterChangesCount(*roster1, *roster2) == getRosterChangesCount(*roster2, *roster1))

	// Check the number of changes is correct, i.e. equal to the cardinality of the difference between the two rosters
	changesCount := getRosterChangesCount(*roster1, *roster2)
	assert.True(t, changesCount == i+j)

	cpy := copyRoster(roster1)
	cpy.List = cpy.List[:len(cpy.List)-1]
	changesCount = getRosterChangesCount(*roster1, *cpy)
	assert.True(t, changesCount == 1)

	// Check the number of changes is 0 when the parameters are the same roster
	changesCount = getRosterChangesCount(*roster1, *roster1)
	assert.True(t, changesCount == 0)

	// Check the number of changes is the cardinality of the non-empty roster when given an empty and a non-empty roster
	empty := copyRoster(roster1)
	empty.List = make([]*network.ServerIdentity, 0)
	changesCount = getRosterChangesCount(*empty, *roster1)
	assert.True(t, changesCount == len(roster1.List))
}

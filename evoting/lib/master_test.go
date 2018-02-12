package lib

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/dedis/cothority/skipchain"

	"github.com/stretchr/testify/assert"
)

func TestFetchMaster(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	_, err := FetchMaster(roster, []byte{})
	assert.NotNil(t, err)

	master := &Master{Roster: roster}
	master.GenChain([]byte{0}, []byte{1})

	m, _ := FetchMaster(roster, master.ID)
	assert.Equal(t, master.ID, m.ID)
}

func TestLinks(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	master := &Master{Roster: roster}
	master.GenChain([]byte{0}, []byte{1})

	links, _ := master.Links()
	assert.Equal(t, 2, len(links))
	assert.Equal(t, skipchain.SkipBlockID([]byte{0}), links[0].ID)
	assert.Equal(t, skipchain.SkipBlockID([]byte{1}), links[1].ID)
}

func TestIsAdmin(t *testing.T) {
	m := &Master{Admins: []uint32{0}}
	assert.True(t, m.IsAdmin(0))
	assert.False(t, m.IsAdmin(1))
}

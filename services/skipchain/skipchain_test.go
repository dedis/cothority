package skipchain

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestSkipBlockData_Hash(t *testing.T) {
	sbd1 := &SkipBlockData{
		SkipBlockCommon: &SkipBlockCommon{
			Height: 1,
		},
		Data: []byte("1"),
	}
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := &SkipBlockData{
		SkipBlockCommon: &SkipBlockCommon{
			Height: 2,
		},
		Data: []byte("2"),
	}
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlockRoster_Hash(t *testing.T) {
	local := sda.NewLocalTest()
	hosts, el, _ := local.GenTree(2, false, false, false)
	defer local.CloseAll()
	sbd1 := &SkipBlockRoster{
		SkipBlockCommon: &SkipBlockCommon{
			Height: 1,
		},
		RosterName: "genesis",
		EntityList: el,
	}
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := &SkipBlockRoster{
		SkipBlockCommon: &SkipBlockCommon{
			Height: 1,
		},
		RosterName: "genesis",
		EntityList: local.GenEntityListFromHost(hosts[0]),
	}
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestService_ProposeSkipBlock(t *testing.T) {
	// send a ProposeBlock
	genesis := &SkipBlockData{
		Data: []byte("In the beginning God created the heaven and the earth."),
		SkipBlockCommon: &SkipBlockCommon{
			MaximumHeight: 2,
		},
	}
	blockCount := 0
	s := newSkipchainService(nil, "").(*Service)
	psbr, err := s.ProposeSkipBlock(nil, genesis)
	assert.Nil(t, err)
	latest := psbr.Latest.(*SkipBlockData)
	// verify creation of GenesisBlock:
	blockCount++
	assert.Equal(t, blockCount, latest.Index)
	assert.Equal(t, 1, len(latest.BackLink))
	assert.NotEqual(t, 0, latest.BackLink)

	next := &SkipBlockData{
		Data: []byte("And the earth was without form, and void; " +
			"and darkness was upon the face of the deep. " +
			"And the Spirit of God moved upon the face of the waters."),
		SkipBlockCommon: &SkipBlockCommon{
			MaximumHeight: 2,
		},
	}
	psbr2, err := s.ProposeSkipBlock(genesis.Hash, next)
	latest = psbr2.Latest.(*SkipBlockData)
	// verify creation of GenesisBlock:
	blockCount++
	assert.Equal(t, blockCount, latest.Index)
	assert.Equal(t, 1, len(latest.BackLink))
	assert.NotEqual(t, 0, latest.BackLink)
}

func TestService_GetUpdateChain(t *testing.T) {
}

package skipchain

import (
	"testing"

	"bytes"

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
		EntityList: el,
	}
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := &SkipBlockRoster{
		SkipBlockCommon: &SkipBlockCommon{
			Height: 1,
		},
		EntityList: local.GenEntityListFromHost(hosts[0]),
	}
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlockInterface(t *testing.T) {
	// Tests the different accessors
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
	// Create a small chain and test whether we can get from one element
	// of the chain to the last element with a valid slice of SkipBlocks
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, el, s := makeHELS(local, 4)
	sbLength := 10
	sbs := make([]*SkipBlockRoster, sbLength)
	sbs[0] = makeGenesisRoster(s, el)
	for i := 1; i < sbLength-1; i++ {
		el.List = el.List[0 : sbLength-i]
		reply, err := s.ProposeSkipBlock(sbs[i-1].Hash,
			&SkipBlockRoster{EntityList: el})
		dbg.ErrFatal(err)
		sbs[i] = reply.Latest.(*SkipBlockRoster)
	}
	for i := 0; i < sbLength; i++ {
		sbc, err := s.GetUpdateChain(sbs[i].Hash)
		dbg.ErrFatal(err)
		if !bytes.Equal(sbc.Update[0].GetCommon().Hash,
			sbs[i].Hash) {
			t.Fatal("First hash is not from our SkipBlock")
		}
		if !bytes.Equal(sbc.Update[len(sbc.Update)-1].GetCommon().Hash,
			sbs[sbLength-1].Hash) {
			t.Fatal("Last Hash is not equal to last SkipBlock")
		}
		for up, sb1 := range sbc.Update {
			dbg.ErrFatal(sb1.VerifySignatures())
			if up < len(sbc.Update)-1 {
				sb2 := sbc.Update[up]
				h1 := sb1.GetCommon().Height
				h2 := sb2.GetCommon().Height
				// height := min(h1, h2)
				height := h1
				if h2 < height {
					height = h2
				}
				if !bytes.Equal(sb1.GetCommon().ForwardLink[height].Hash,
					sb2.GetCommon().Hash) {
					t.Fatal("Forward-pointer of", up,
						"is different of hash in", up+1)
				}
			}
		}
	}
}

func TestService_PropagateSkipBlock(t *testing.T) {
}

func TestService_ForwardSignature(t *testing.T) {
}

// makes a genesis Roster-block
func makeGenesisRoster(s *Service, el *sda.EntityList) *SkipBlockRoster {
	sb := &SkipBlockRoster{
		SkipBlockCommon: &SkipBlockCommon{
			MaximumHeight: 4,
		},
		EntityList: el,
	}
	reply, err := s.ProposeSkipBlock(nil, sb)
	dbg.ErrFatal(err)
	return reply.Latest.(*SkipBlockRoster)
}

// Makes a Host, an EntityList, and a service
func makeHELS(local *sda.LocalTest, nbr int) ([]*sda.Host, *sda.EntityList, *Service) {
	hosts := local.GenLocalHosts(nbr, false, false)
	el := local.GenEntityListFromHost(hosts...)
	return hosts, el, local.Services[hosts[0].Entity.ID][sda.ServiceFactory.ServiceID("Skipchain")].(*Service)
}

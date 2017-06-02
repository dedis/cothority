package skipchain_test

import (
	"testing"

	"fmt"

	"github.com/dedis/cothority/skipchain"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

var invalidID = skipchain.SkipBlockID([]byte{1, 2, 3})

func TestSkipBlockBunch_Length(t *testing.T) {
	tb := newTestBunch(3, 2, 1)
	defer tb.End()

	require.Equal(t, 2, tb.bunch.Length())
}

func TestSkipBlockBunch_GetResponsible(t *testing.T) {
	tb := newTestBunch(3, 2, 1)
	defer tb.End()

	_, err := tb.bunch.GetResponsible(nil)
	require.NotNil(t, err)

	_, err = tb.bunch.GetResponsible(tb.genesis)
	require.Nil(t, err)
	_, err = tb.bunch.GetResponsible(tb.skipblocks[1])
	require.Nil(t, err)

	// Create a fake child-skipchain
	g := tb.bunch.GetByID(tb.bunch.GenesisID).Copy()
	g.ParentBlockID = invalidID
	g.Roster = nil

	_, err = tb.bunch.GetResponsible(g)
	require.NotNil(t, err)

	bl := tb.bunch.Latest.Copy()
	bl.BackLinkIDs = []skipchain.SkipBlockID{}
	_, err = tb.bunch.GetResponsible(bl)
	require.NotNil(t, err)
	bl.BackLinkIDs = []skipchain.SkipBlockID{invalidID}
	_, err = tb.bunch.GetResponsible(bl)
	require.NotNil(t, err)
}

func TestSkipBlockBunch_Parents(t *testing.T) {
	tb := newTestBunch(3, 2, 1)
	defer tb.End()

	newSC, cerr := tb.client.CreateGenesis(tb.roster, 1, 1, skipchain.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	tb.bunch.Store(newSC)
}

func TestSkipBlockBunch_VerifyLinks(t *testing.T) {
	tb := newTestBunch(3, 2, 4)
	defer tb.End()

	err := tb.bunch.VerifyLinks(tb.genesis)
	log.ErrFatal(err)

	g := tb.genesis.Copy()
	g.ForwardLink[0].Hash = []byte{}
	require.NotNil(t, tb.bunch.VerifyLinks(g))

	g.BackLinkIDs = []skipchain.SkipBlockID{}
	require.NotNil(t, tb.bunch.VerifyLinks(g))

	last := tb.bunch.Latest.Copy()
	last.BackLinkIDs = []skipchain.SkipBlockID{invalidID}
	require.NotNil(t, tb.bunch.VerifyLinks(last))

	last = tb.bunch.Latest.Copy()
	last.Hash = invalidID
	require.NotNil(t, tb.bunch.VerifyLinks(last))

}

func TestSkipBlockBunch_GetFuzzy(t *testing.T) {
	tb := newTestBunch(3, 2, 0)
	defer tb.End()

	id := fmt.Sprintf("%x", tb.genesis.Hash)
	require.NotNil(t, tb.bunch.GetFuzzy(id))
	require.NotNil(t, tb.bunch.GetFuzzy(id[0:10]))
	require.NotNil(t, tb.bunch.GetFuzzy(id[10:]))
	require.NotNil(t, tb.bunch.GetFuzzy(id[10:20]))
	require.Nil(t, tb.bunch.GetFuzzy(fmt.Sprintf("%x", invalidID)))
}

func TestSBBStorage_AddBunch(t *testing.T) {
	tb := newTestBunch(3, 2, 0)
	defer tb.End()

	sbbs := skipchain.NewSBBStorage()
	require.NotNil(t, sbbs.AddBunch(tb.genesis))
	require.Nil(t, sbbs.AddBunch(tb.genesis))
}

func TestSBBStorage_Store(t *testing.T) {
	tb := newTestBunch(3, 2, 0)
	defer tb.End()

	newGen, cerr := tb.client.CreateGenesis(tb.roster, 2, 2, skipchain.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	tb.storage.Store(newGen)
}

func TestSBBStorage_GetFuzzy(t *testing.T) {
	tb := newTestBunch(3, 2, 0)
	defer tb.End()

	sbbs := tb.storage
	id := fmt.Sprintf("%x", tb.genesis.Hash)
	require.NotNil(t, sbbs.GetFuzzy(id))
	require.NotNil(t, sbbs.GetFuzzy(id[0:10]))
	require.NotNil(t, sbbs.GetFuzzy(id[10:]))
	require.NotNil(t, sbbs.GetFuzzy(id[10:20]))
	require.Nil(t, sbbs.GetFuzzy(fmt.Sprintf("%x", invalidID)))
}

func TestSBBStorage_GetFromGenesisByID(t *testing.T) {
	tb := newTestBunch(3, 2, 0)
	defer tb.End()

	sb := tb.storage.GetFromGenesisByID(tb.genesis.Hash, tb.genesis.Hash)
	require.True(t, sb.Equal(tb.genesis))
}

func TestSBBStorage_GetLatest(t *testing.T) {
	tb := newTestBunch(3, 2, 0)
	defer tb.End()

	sb := tb.storage.GetLatest(tb.genesis.Hash)
	require.True(t, sb.Equal(tb.bunch.Latest))
}

func TestNewSBBStorage_VerifyLinks(t *testing.T) {
	tb := newTestBunch(3, 2, 2)
	defer tb.End()

	for _, sb := range tb.skipblocks {
		log.ErrFatal(tb.storage.VerifyLinks(sb))
	}

	child, cerr := tb.client.CreateGenesis(tb.roster, 1, 1, skipchain.VerificationNone,
		nil, tb.bunch.GenesisID)
	require.NotNil(t, tb.storage.VerifyLinks(child))
	cc := child.Copy()
	cc.BackLinkIDs = []skipchain.SkipBlockID{}
	require.NotNil(t, tb.storage.VerifyLinks(cc))

	tb.storage.Store(child)
	log.ErrFatal(cerr)
	log.ErrFatal(tb.storage.VerifyLinks(child))

	cc = child.Copy()
	cc.ParentBlockID = invalidID
	require.NotNil(t, tb.storage.VerifyLinks(cc))

	cc = child.Copy()
	cc.Hash = invalidID
	tb.storage.Store(cc)
	require.NotNil(t, tb.storage.VerifyLinks(cc))
}

type testBunch struct {
	local      *onet.LocalTest
	client     *skipchain.Client
	roster     *onet.Roster
	servers    []*onet.Server
	genesis    *skipchain.SkipBlock
	skipblocks []*skipchain.SkipBlock
	bunch      *skipchain.SkipBlockBunch
	storage    *skipchain.SBBStorage
}

func newTestBunch(nbrHosts, height, nbrAddSB int) *testBunch {
	tb := &testBunch{
		skipblocks: make([]*skipchain.SkipBlock, nbrAddSB+1),
		storage:    skipchain.NewSBBStorage(),
	}
	tb.local = onet.NewTCPTest()
	tb.servers, tb.roster, _ = tb.local.GenTree(nbrHosts, true)

	tb.client = skipchain.NewClient()
	log.Lvl2("Creating root and control chain")
	var cerr onet.ClientError
	tb.genesis, cerr = tb.client.CreateGenesis(tb.roster, height, height, skipchain.VerificationNone, nil, nil)
	tb.skipblocks[0] = tb.genesis
	tb.bunch = skipchain.NewSkipBlockBunch(tb.genesis)
	log.ErrFatal(cerr)
	for i := 1; i <= nbrAddSB; i++ {
		log.Lvl2("Creating skipblock", i+1)
		var cerr onet.ClientError
		_, tb.skipblocks[i], cerr = tb.client.AddSkipBlock(tb.skipblocks[i-1], nil, nil)
		tb.bunch.Store(tb.skipblocks[i])
		log.ErrFatal(cerr)
	}
	// Get all skipblocks with all updated forward-links
	for i, sb := range tb.skipblocks {
		sbNew, cerr := tb.client.GetSingleBlock(tb.roster, sb.Hash)
		log.ErrFatal(cerr)
		tb.skipblocks[i] = sbNew
		tb.bunch.Store(sbNew)
		tb.storage.Store(sbNew)
	}
	return tb
}

func (tb *testBunch) End() {
	tb.WaitPropagated(tb.skipblocks)
	sbs := []*skipchain.SkipBlock{}
	for _, sb := range tb.bunch.SkipBlocks {
		sbs = append(sbs, sb)
	}
	tb.WaitPropagated(sbs)
	tb.local.CloseAll()
}

func (tb *testBunch) WaitPropagated(sbs []*skipchain.SkipBlock) {
	allPropagated := false
	for !allPropagated {
		allPropagated = true
		for _, sb := range sbs {
			for _, si := range tb.roster.List {
				reply := &skipchain.GetBlocksReply{}
				gb := &skipchain.GetBlocks{Start: nil, End: sb.Hash, MaxHeight: 0}
				cerr := tb.client.SendProtobuf(si, gb, reply)
				if cerr != nil {
					log.Error(cerr)
					allPropagated = false
				} else if len(reply.Reply) == 0 {
					log.LLvl3("no block yet")
					allPropagated = false
				} else if !sb.Equal(reply.Reply[0]) {
					log.LLvl3("not same block")
					allPropagated = false
				}
			}
		}
	}
}

package skipchain_test

import (
	"testing"

	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

const tsName = "tsName"

var tsID onet.ServiceID
var tSuite = skipchain.Suite

func init() {
	var err error
	tsID, err = onet.RegisterNewService(tsName, newTestService)
	log.ErrFatal(err)
}

// TestGB tests the GetBlocks protocol
func TestGB(t *testing.T) {
	log.SetDebugVisible(3)

	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	servers, ro, _ := local.GenTree(3, true)
	tss := local.GetServices(servers, tsID)

	ts0 := tss[0].(*testService)
	ts1 := tss[1].(*testService)
	sb0 := skipchain.NewSkipBlock()
	sb0.Roster = ro
	sb0.Hash = sb0.CalculateHash()
	sb1 := skipchain.NewSkipBlock()
	sb1.BackLinkIDs = []skipchain.SkipBlockID{sb0.Hash}
	sb1.Hash = sb1.CalculateHash()
	bl := &skipchain.BlockLink{BFTSignature: bftcosi.BFTSignature{Msg: sb1.Hash, Sig: []byte{}}}
	sb0.ForwardLink = []*skipchain.BlockLink{bl}
	db, bucket := ts0.GetAdditionalBucket("skipblocks")
	ts0.Db = skipchain.NewSkipBlockDB(db, bucket)
	ts0.Db.Store(sb0)
	ts0.Db.Store(sb1)
	db, bucket = ts1.GetAdditionalBucket("skipblocks")
	ts1.Db = skipchain.NewSkipBlockDB(db, bucket)
	ts1.Db.Store(sb0)
	ts1.Db.Store(sb1)

	// ask for only one
	sb := ts1.CallGB(sb0, 1)
	require.NotNil(t, sb)
	require.Equal(t, 1, len(sb))
	require.Equal(t, sb0.Hash, sb[0].Hash)

	// In order to test GetUpdate in the face of failures, pause one
	servers[2].Pause()

	// ask for 10, expect to get the 2 be put in above.
	sb = ts1.CallGB(sb0, 10)
	require.NotNil(t, sb)
	require.Equal(t, 2, len(sb))
	require.Equal(t, sb0.Hash, sb[0].Hash)
	require.Equal(t, sb1.Hash, sb[1].Hash)

	// And what about getupdate with all servers replying?
	// server[2] does not have the correct block in it, so we expect it
	// to get the request, but send no reply back. One of the others
	// will find the blocks.
	servers[2].Unpause()
	sb = ts1.CallGB(sb0, 10)
	require.NotNil(t, sb)
	require.Equal(t, 2, len(sb))
	require.Equal(t, sb0.Hash, sb[0].Hash)
	require.Equal(t, sb1.Hash, sb[1].Hash)
}

// TestER tests the ProtoExtendRoster message
func TestER(t *testing.T) {
	nodes := []int{2, 5, 13}
	for _, nbrNodes := range nodes {
		testER(t, tsID, nbrNodes)
	}
}

func testER(t *testing.T, tsid onet.ServiceID, nbrNodes int) {
	log.Lvl1("Testing", nbrNodes, "nodes")
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	servers, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	tss := local.GetServices(servers, tsid)
	log.Lvl3(tree.Dump())

	sb := &skipchain.SkipBlock{SkipBlockFix: &skipchain.SkipBlockFix{Roster: roster,
		Data: []byte{}}}

	// Check refusing of new chains
	for _, t := range tss {
		t.(*testService).FollowerIDs = []skipchain.SkipBlockID{[]byte{0}}
	}
	ts := tss[0].(*testService)
	sigs := ts.CallER(tree, sb)
	require.Equal(t, 0, len(sigs))

	// Check inclusion of new chains
	for _, t := range tss {
		t.(*testService).Followers = []skipchain.FollowChainType{{
			Block:    sb,
			NewChain: skipchain.NewChainAnyNode,
		}}
	}
	sigs = ts.CallER(tree, sb)
	require.True(t, len(sigs)+(nbrNodes-1)/3 >= nbrNodes-1)

	for _, s := range sigs {
		_, si := roster.Search(s.SI)
		require.NotNil(t, si)
		require.Nil(t, schnorr.Verify(tSuite, si.Public, sb.SkipChainID(), s.Signature))
	}

	// When only one node refuse,
	// we should be able to proceed because skipchain is fault tolerant
	if nbrNodes > 4 {
		for i := 3; i < nbrNodes; i++ {
			log.Lvl2("Checking failing signature at", i)
			tss[i].(*testService).FollowerIDs = []skipchain.SkipBlockID{[]byte{0}}
			tss[i].(*testService).Followers = []skipchain.FollowChainType{}
			sigs = ts.CallER(tree, sb)
			require.Equal(t, 0, len(sigs))
			tss[i].(*testService).Followers = []skipchain.FollowChainType{{
				Block:    sb,
				NewChain: skipchain.NewChainAnyNode,
			}}
		}
	}
}

type testService struct {
	*onet.ServiceProcessor
	Followers   []skipchain.FollowChainType
	FollowerIDs []skipchain.SkipBlockID
	Db          *skipchain.SkipBlockDB
}

func (ts *testService) CallER(t *onet.Tree, b *skipchain.SkipBlock) []skipchain.ProtoExtendSignature {
	pi, err := ts.CreateProtocol(skipchain.ProtocolExtendRoster, t)
	if err != nil {
		return []skipchain.ProtoExtendSignature{}
	}
	pisc := pi.(*skipchain.ExtendRoster)
	pisc.ExtendRoster = &skipchain.ProtoExtendRoster{Block: *b}
	if err := pi.Start(); err != nil {
		log.ErrFatal(err)
	}
	return <-pisc.ExtendRosterReply
}

func (ts *testService) CallGB(sb *skipchain.SkipBlock, n int) []*skipchain.SkipBlock {
	t := sb.Roster.RandomSubset(ts.ServerIdentity(), 3).GenerateStar()
	log.Lvl3("running on this tree", t.Dump())

	pi, err := ts.CreateProtocol(skipchain.ProtocolGetBlocks, t)
	if err != nil {
		log.Error(err)
		return nil
	}
	pisc := pi.(*skipchain.GetBlocks)
	pisc.GetBlocks = &skipchain.ProtoGetBlocks{
		Count: n,
		SBID:  sb.Hash,
	}
	if err := pi.Start(); err != nil {
		log.ErrFatal(err)
	}
	result := <-pisc.GetBlocksReply
	return result
}

func (ts *testService) NewProtocol(ti *onet.TreeNodeInstance, conf *onet.GenericConfig) (pi onet.ProtocolInstance, err error) {
	if ti.ProtocolName() == skipchain.ProtocolExtendRoster {
		// Start by getting latest blocks of all followers
		pi, err = skipchain.NewProtocolExtendRoster(ti)
		if err == nil {
			pier := pi.(*skipchain.ExtendRoster)
			pier.Followers = &ts.Followers
			pier.FollowerIDs = ts.FollowerIDs
			pier.DB = ts.Db
		}
	}
	if ti.ProtocolName() == skipchain.ProtocolGetBlocks {
		pi, err = skipchain.NewProtocolGetBlocks(ti)
		if err == nil {
			pigu := pi.(*skipchain.GetBlocks)
			pigu.DB = ts.Db
		}
	}
	return
}

func newTestService(c *onet.Context) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s, nil
}

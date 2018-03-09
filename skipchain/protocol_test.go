package skipchain_test

import (
	"sync"
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoinx"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

const tsName = "tsName"

var tsID onet.ServiceID

func init() {
	var err error
	tsID, err = onet.RegisterNewService(tsName, newTestService)
	log.ErrFatal(err)
}

// TestGB tests the GetBlocks protocol
func TestGB(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers, ro, _ := local.GenTree(3, true)
	tss := local.GetServices(servers, tsID)

	ts0 := tss[0].(*testService)
	ts1 := tss[1].(*testService)
	ts2 := tss[2].(*testService)

	sb0 := skipchain.NewSkipBlock()
	sb0.Roster = ro
	sb0.Hash = sb0.CalculateHash()
	sb1 := skipchain.NewSkipBlock()
	sb1.Roster = ro
	sb1.BackLinkIDs = []skipchain.SkipBlockID{sb0.Hash}
	sb1.Hash = sb1.CalculateHash()
	sig0 := skipchain.NewForwardLink(sb0, sb1)
	sig0.Signature = byzcoinx.FinalSignature{Msg: sig0.Hash(), Sig: []byte{}}
	sb0.ForwardLink = []*skipchain.ForwardLink{sig0}

	sb2 := skipchain.NewSkipBlock()
	sb2.BackLinkIDs = []skipchain.SkipBlockID{sb1.Hash}
	sb2.Hash = sb2.CalculateHash()

	sb3 := skipchain.NewSkipBlock()
	sb3.BackLinkIDs = []skipchain.SkipBlockID{sb2.Hash}
	sb3.Hash = sb3.CalculateHash()
	sig2 := skipchain.NewForwardLink(sb2, sb3)
	sig2.Signature = byzcoinx.FinalSignature{Msg: sig2.Hash(), Sig: []byte{}}
	sb2.ForwardLink = []*skipchain.ForwardLink{sig2}

	// and make sb1 forward[1] point to sb3 as well.
	sig12 := skipchain.NewForwardLink(sb1, sb2)
	sig12.Signature = byzcoinx.FinalSignature{Msg: sig12.Hash(), Sig: []byte{}}

	sig13 := skipchain.NewForwardLink(sb1, sb3)
	sig13.Signature = byzcoinx.FinalSignature{Msg: sig13.Hash(), Sig: []byte{}}
	sb1.ForwardLink = []*skipchain.ForwardLink{sig12, sig13}

	db, bucket := ts0.GetAdditionalBucket([]byte("skipblocks"))
	ts0.Db = skipchain.NewSkipBlockDB(db, bucket)
	ts0.Db.Store(sb0)
	ts0.Db.Store(sb1)
	ts0.Db.Store(sb2)
	ts0.Db.Store(sb3)
	db, bucket = ts1.GetAdditionalBucket([]byte("skipblocks"))
	ts1.Db = skipchain.NewSkipBlockDB(db, bucket)
	ts1.Db.Store(sb0)
	ts1.Db.Store(sb1)
	ts1.Db.Store(sb2)
	ts1.Db.Store(sb3)
	ts2.Db = skipchain.NewSkipBlockDB(db, bucket)
	// do not save anything into ts2 so that
	// it is totally out of date, and cannot answer anything

	// ask for only one
	sb := ts1.CallGB(sb0, false, 1)
	require.NotNil(t, sb)
	require.Equal(t, 1, len(sb))
	require.Equal(t, sb0.Hash, sb[0].Hash)

	// In order to test GetUpdate in the face of failures, pause one
	servers[2].Pause()

	// ask for 10, expect to get the 4 of them
	sb = ts1.CallGB(sb0, false, 10)
	require.NotNil(t, sb)
	require.Equal(t, 4, len(sb))
	require.Equal(t, sb0.Hash, sb[0].Hash)
	require.Equal(t, sb1.Hash, sb[1].Hash)
	require.Equal(t, sb2.Hash, sb[2].Hash)
	require.Equal(t, sb3.Hash, sb[3].Hash)

	// ask for 3
	sb = ts1.CallGB(sb1, false, 3)
	require.NotNil(t, sb)
	require.Equal(t, 3, len(sb))
	require.Equal(t, sb1.Hash, sb[0].Hash)
	require.Equal(t, sb2.Hash, sb[1].Hash)
	require.Equal(t, sb3.Hash, sb[2].Hash)

	// And what about getupdate with all servers replying?
	// server[2] does not have the correct block in it, so we expect it
	// to get the request, but send no reply back. One of the others
	// will find the blocks.
	servers[2].Unpause()
	sb = ts1.CallGB(sb0, false, 10)
	require.NotNil(t, sb)
	require.Equal(t, 4, len(sb))
	require.Equal(t, sb0.Hash, sb[0].Hash)
	require.Equal(t, sb1.Hash, sb[1].Hash)
	require.Equal(t, sb2.Hash, sb[2].Hash)
	require.Equal(t, sb3.Hash, sb[3].Hash)

	// with skipping, we expect to get sb0, sb1 and sb3.
	sb = ts1.CallGB(sb0, true, 10)
	require.NotNil(t, sb)
	require.Equal(t, 3, len(sb))
	require.Equal(t, sb0.Hash, sb[0].Hash)
	require.Equal(t, sb1.Hash, sb[1].Hash)
	require.Equal(t, sb3.Hash, sb[2].Hash)

}

// TestER tests the ProtoExtendRoster message
func TestER(t *testing.T) {
	nodes := []int{2, 5, 13}
	for _, nbrNodes := range nodes {
		testER(t, tsID, nbrNodes)
	}
}

func TestFail(t *testing.T) {
	if testing.Short() {
		t.Skip("Stress-test of localtest with premature close")
	}
	for i := 0; i < 50; i++ {
		log.Lvl1("Starting test", i)
		nbrNodes := 10
		local := onet.NewLocalTest(cothority.Suite)
		servers, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
		tss := local.GetServices(servers, tsID)

		sb := &skipchain.SkipBlock{SkipBlockFix: &skipchain.SkipBlockFix{Roster: roster,
			Data: []byte{}}}

		// Check refusing of new chains
		for _, t := range tss {
			t.(*testService).FollowerIDs = []skipchain.SkipBlockID{[]byte{0}}
		}
		sigs := tss[0].(*testService).CallER(tree, sb)
		require.Equal(t, 0, len(sigs))
		local.CloseAll()
	}
}

func testER(t *testing.T, tsid onet.ServiceID, nbrNodes int) {
	log.Lvl1("Testing", nbrNodes, "nodes")
	local := onet.NewLocalTest(cothority.Suite)
	servers, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	tss := local.GetServices(servers, tsid)

	sb := &skipchain.SkipBlock{SkipBlockFix: &skipchain.SkipBlockFix{Roster: roster,
		Data: []byte{}}}

	// Check refusing of new chains
	for _, t := range tss {
		t.(*testService).FollowerIDs = []skipchain.SkipBlockID{[]byte{0}}
	}
	sigs := tss[0].(*testService).CallER(tree, sb)
	require.Equal(t, 0, len(sigs))
	local.CloseAll()

	// Check inclusion of new chains
	local = onet.NewLocalTest(cothority.Suite)
	servers, roster, tree = local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	tss = local.GetServices(servers, tsid)
	for _, t := range tss {
		t.(*testService).Followers = &[]skipchain.FollowChainType{{
			Block:    sb,
			NewChain: skipchain.NewChainAnyNode,
		}}
	}
	sigs = tss[0].(*testService).CallER(tree, sb)
	require.True(t, len(sigs)+(nbrNodes-1)/3 >= nbrNodes-1)

	for _, s := range sigs {
		_, si := roster.Search(s.SI)
		require.NotNil(t, si)
		require.Nil(t, schnorr.Verify(cothority.Suite, si.Public, sb.SkipChainID(), s.Signature))
	}
	local.CloseAll()

	// When only one node refuses,
	// we should be able to proceed because skipchain is fault tolerant
	if nbrNodes > 4 {
		local = onet.NewLocalTest(cothority.Suite)
		servers, roster, tree = local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
		tss = local.GetServices(servers, tsid)
		for i := 3; i < nbrNodes; i++ {
			log.Lvl2("Checking failing signature at", i)
			tss[i].(*testService).Lock()
			tss[i].(*testService).FollowerIDs = []skipchain.SkipBlockID{[]byte{0}}
			tss[i].(*testService).Followers = &[]skipchain.FollowChainType{}
			tss[i].(*testService).Unlock()
			sigs = tss[0].(*testService).CallER(tree, sb)
			require.Equal(t, 0, len(sigs))
			tss[i].(*testService).Lock()
			tss[i].(*testService).Followers = &[]skipchain.FollowChainType{{
				Block:    sb,
				NewChain: skipchain.NewChainAnyNode,
			}}
			tss[i].(*testService).Unlock()
		}
		local.CloseAll()
	}
}

type testService struct {
	*onet.ServiceProcessor
	sync.Mutex
	Followers   *[]skipchain.FollowChainType
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

func (ts *testService) CallGB(sb *skipchain.SkipBlock, sk bool, n int) []*skipchain.SkipBlock {
	t := sb.Roster.RandomSubset(ts.ServerIdentity(), 3).GenerateStar()
	log.Lvl3("running on this tree", t.Dump())

	pi, err := ts.CreateProtocol(skipchain.ProtocolGetBlocks, t)
	if err != nil {
		log.Error(err)
		return nil
	}
	pisc := pi.(*skipchain.GetBlocks)
	pisc.GetBlocks = &skipchain.ProtoGetBlocks{
		Count:    n,
		SBID:     sb.Hash,
		Skipping: sk,
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
			ts.Lock()
			pier.Followers = ts.Followers
			pier.FollowerIDs = ts.FollowerIDs
			pier.DB = ts.Db
			ts.Unlock()
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

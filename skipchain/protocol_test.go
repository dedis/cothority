package skipchain

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
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

	sb0 := NewSkipBlock()
	sb0.Roster = ro
	sb0.Index = 0
	sb0.Height = 1
	sb0.Hash = sb0.CalculateHash()
	sb1 := NewSkipBlock()
	sb1.Roster = ro
	sb1.Index = 1
	sb1.Height = 2
	sb1.BackLinkIDs = []SkipBlockID{sb0.Hash}
	sb1.Hash = sb1.CalculateHash()
	sig0 := NewForwardLink(sb0, sb1)
	require.NoError(t, sig0.sign(ro))
	sb0.ForwardLink = []*ForwardLink{sig0}

	sb2 := NewSkipBlock()
	sb2.Roster = ro
	sb2.Index = 2
	sb2.Height = 1
	sb2.BackLinkIDs = []SkipBlockID{sb1.Hash}
	sb2.Hash = sb2.CalculateHash()

	sb3 := NewSkipBlock()
	sb3.Roster = ro
	sb3.Index = 3
	sb3.Height = 1
	sb3.BackLinkIDs = []SkipBlockID{sb2.Hash}
	sb3.Hash = sb3.CalculateHash()
	sig2 := NewForwardLink(sb2, sb3)
	require.NoError(t, sig2.sign(ro))
	sb2.ForwardLink = []*ForwardLink{sig2}

	// and make sb1 forward[1] point to sb3 as well.
	sig12 := NewForwardLink(sb1, sb2)
	require.NoError(t, sig12.sign(ro))

	sig13 := NewForwardLink(sb1, sb3)
	require.NoError(t, sig13.sign(ro))
	sb1.ForwardLink = []*ForwardLink{sig12, sig13}

	db, bucket := ts0.GetAdditionalBucket([]byte("skipblocks"))
	ts0.Db = NewSkipBlockDB(db, bucket)
	db, bucket = ts1.GetAdditionalBucket([]byte("skipblocks"))
	ts1.Db = NewSkipBlockDB(db, bucket)
	ts2.Db = NewSkipBlockDB(db, bucket)
	blocks := []*SkipBlock{sb0, sb1, sb2, sb3}
	_, err := ts0.Db.StoreBlocks(blocks)
	require.Nil(t, err)
	_, err = ts1.Db.StoreBlocks(blocks)
	require.Nil(t, err)
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

func testER(t *testing.T, tsid onet.ServiceID, nbrNodes int) {
	log.Lvl1("Testing", nbrNodes, "nodes")
	local := onet.NewLocalTest(cothority.Suite)
	servers, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	tss := local.GetServices(servers, tsid)

	sb := &SkipBlock{SkipBlockFix: &SkipBlockFix{Roster: roster,
		Data: []byte{}}}

	// Check refusing of new chains
	for _, t := range tss {
		t.(*testService).FollowerIDs = []SkipBlockID{[]byte{0}}
	}
	sigs := tss[0].(*testService).CallER(tree, sb)
	require.Equal(t, 0, len(sigs))
	local.CloseAll()

	// Check inclusion of new chains
	local = onet.NewLocalTest(cothority.Suite)
	servers, roster, tree = local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	tss = local.GetServices(servers, tsid)
	for _, t := range tss {
		t.(*testService).Followers = &[]FollowChainType{{
			Block:    sb,
			NewChain: NewChainAnyNode,
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
		servers, _, tree = local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
		tss = local.GetServices(servers, tsid)
		for i := 3; i < nbrNodes; i++ {
			log.Lvl2("Checking failing signature at", i)
			tss[i].(*testService).Lock()
			tss[i].(*testService).FollowerIDs = []SkipBlockID{[]byte{0}}
			tss[i].(*testService).Followers = &[]FollowChainType{}
			tss[i].(*testService).Unlock()
			sigs = tss[0].(*testService).CallER(tree, sb)
			require.Equal(t, 0, len(sigs))
			tss[i].(*testService).Lock()
			tss[i].(*testService).Followers = &[]FollowChainType{{
				Block:    sb,
				NewChain: NewChainAnyNode,
			}}
			tss[i].(*testService).Unlock()
		}
		local.CloseAll()
	}
}

type testService struct {
	*onet.ServiceProcessor
	sync.Mutex
	Followers   *[]FollowChainType
	FollowerIDs []SkipBlockID
	Db          *SkipBlockDB
}

func (ts *testService) CallER(t *onet.Tree, b *SkipBlock) []ProtoExtendSignature {
	pi, err := ts.CreateProtocol(ProtocolExtendRoster, t)
	if err != nil {
		return []ProtoExtendSignature{}
	}
	pisc := pi.(*ExtendRoster)
	pisc.ExtendRoster = &ProtoExtendRoster{Block: *b}
	if err := pi.Start(); err != nil {
		log.ErrFatal(err)
	}
	return <-pisc.ExtendRosterReply
}

func (ts *testService) CallGB(sb *SkipBlock, sk bool, n int) []*SkipBlock {
	t := sb.Roster.RandomSubset(ts.ServerIdentity(), 3).GenerateStar()
	log.Lvl3("running on this tree", t.Dump())

	pi, err := ts.CreateProtocol(ProtocolGetBlocks, t)
	if err != nil {
		log.Error(err)
		return nil
	}
	pisc := pi.(*GetBlocks)
	pisc.GetBlocks = &ProtoGetBlocks{
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
	if ti.ProtocolName() == ProtocolExtendRoster {
		// Start by getting latest blocks of all followers
		pi, err = NewProtocolExtendRoster(ti)
		if err == nil {
			pier := pi.(*ExtendRoster)
			ts.Lock()
			pier.Followers = ts.Followers
			pier.FollowerIDs = ts.FollowerIDs
			pier.DB = ts.Db
			ts.Unlock()
		}
	}
	if ti.ProtocolName() == ProtocolGetBlocks {
		pi, err = NewProtocolGetBlocks(ti)
		if err == nil {
			pigu := pi.(*GetBlocks)
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

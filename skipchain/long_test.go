// +build long_test

package skipchain

// Put these very long tests (> 5 minutes) in a separate file
// that will only be built by `go test -tags long_test`.

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

func TestService_ParallelStoreBlock(t *testing.T) {
	nbrRoutines := 20
	numBlocks := 100
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, roster, s1 := makeHELS(local, 5)
	ssb := &StoreSkipBlock{
		NewBlock: &SkipBlock{
			SkipBlockFix: &SkipBlockFix{
				MaximumHeight: 1,
				BaseHeight:    1,
				Roster:        roster,
				Data:          []byte{},
			},
		},
	}
	reply, err := s1.StoreSkipBlock(ssb)
	if err != nil {
		t.Error(err)
	}

	errs := make(chan error, nbrRoutines*numBlocks)

	wg := &sync.WaitGroup{}
	wg.Add(nbrRoutines)
	for i := 0; i < nbrRoutines; i++ {
		go func(sb *SkipBlock) {
			for j := 0; j < numBlocks; j++ {
				_, err := s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: []byte{}, NewBlock: sb})
				if err != nil {
					errs <- err
					break
				}
			}
			wg.Done()
		}(reply.Latest.Copy())
	}
	wg.Wait()

	select {
	case err := <-errs:
		t.Error("got an error", err)
	default:
		t.Log("congratulations, no errors")
	}

	n := s1.db.Length()
	// plus 1 for the genesis block
	if n != numBlocks*nbrRoutines+1 {
		t.Error("num blocks is wrong:", n)
	}
}

func TestClient_ParallelGetUpdateChain(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(5, true)
	defer l.CloseAll()

	clients := make(map[int]*Client)
	for i := range [8]byte{} {
		clients[i] = newTestClient(l)
	}
	sb, err := clients[0].CreateGenesis(ro, 32, 32, VerificationStandard, []byte{})
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	for i := range [128]byte{} {
		wg.Add(1)
		go func(i int) {
			_, err := clients[i%8].GetUpdateChain(sb.Roster, sb.Hash)
			require.NoError(t, err)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestFail(t *testing.T) {
	for i := 0; i < 50; i++ {
		log.Lvl1("Starting test", i)
		nbrNodes := 10
		local := onet.NewLocalTest(cothority.Suite)
		servers, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
		tss := local.GetServices(servers, tsID)

		sb := &SkipBlock{SkipBlockFix: &SkipBlockFix{Roster: roster,
			Data: []byte{}}}

		// Check refusing of new chains
		for _, t := range tss {
			t.(*testService).FollowerIDs = []SkipBlockID{[]byte{0}}
		}
		sigs := tss[0].(*testService).CallER(tree, sb)
		require.Equal(t, 0, len(sigs))
		local.CloseAll()
	}
}

// TestForwardLinks tests that forward links are correctly created for higher height
// when the genesis roster is incomplete and evolves. It will create enough blocks
// for the genesis block to be linked with the latest block and it should be propagated
// to every conode.
func TestForwardLinks(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	srvs, ro, service := l.MakeSRS(cothority.Suite, 5, skipchainSID)
	defer l.CloseAll()

	s := service.(*Service)
	roWithout5 := onet.NewRoster(ro.List[:4])

	sbRoot, err := makeGenesisRosterArgs(s, roWithout5, nil, VerificationStandard, 4, 32)

	sb := NewSkipBlock()
	for i := 0; i < 1024; i++ {
		if i < 20 {
			sb.Roster = roWithout5
		} else {
			sb.Roster = ro
		}

		_, err = s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
		require.NoError(t, err)
	}

	time.Sleep(5 * time.Second)

	for _, srv := range srvs {
		sk := srv.Service(ServiceName).(*Service)
		reply, err := sk.GetUpdateChain(&GetUpdateChain{LatestID: sbRoot.Hash})
		require.NoError(t, err)
		require.Equal(t, 2, len(reply.Update))
	}
}

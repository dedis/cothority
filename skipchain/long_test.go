// +target long_test

package skipchain

// Put these very long tests (> 5 minutes) in a separate file
// that will only be built by `go test -tags long_test`.

import (
	"sync"
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestService_ParallelStoreBlock(t *testing.T) {
	nbrRoutines := 20
	numBlocks := 100
	if testing.Short() {
		nbrRoutines = 10
		numBlocks = 20
	}
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
	if testing.Short() {
		t.Skip("Long run not good for Travis")
	}
	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(5, true)
	defer l.CloseAll()

	clients := make(map[int]*Client)
	for i := range [8]byte{} {
		clients[i] = newTestClient(l)
	}
	_, inter, err := clients[0].CreateRootControl(ro, ro, nil, 1, 1, 1)
	log.ErrFatal(err)

	wg := sync.WaitGroup{}
	for i := range [128]byte{} {
		wg.Add(1)
		go func(i int) {
			_, err := clients[i%8].GetUpdateChain(inter.Roster, inter.Hash, nil)
			log.ErrFatal(err)
			wg.Done()
		}(i)
	}
	wg.Wait()
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

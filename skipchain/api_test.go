package skipchain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"bytes"

	"sync"

	"time"

	"github.com/dedis/cothority/skipchain"
	_ "github.com/dedis/cothority/skipchain/service"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	network.RegisterMessage(&testData{})
}

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestClient_CreateGenesis(t *testing.T) {
	l := onet.NewTCPTest()
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := skipchain.NewClient()
	_, cerr := c.CreateGenesis(roster, 1, 1, skipchain.VerificationNone,
		[]byte{1, 2, 3}, nil)
	require.NotNil(t, cerr)
	_, cerr = c.CreateGenesis(roster, 1, 0, skipchain.VerificationNone,
		&testData{}, nil)
	require.NotNil(t, cerr)
	_, cerr = c.CreateGenesis(roster, 1, 1, skipchain.VerificationNone,
		&testData{}, nil)
	require.Nil(t, cerr)
	time.Sleep(time.Second)
}

func TestClient_GetUpdateChain(t *testing.T) {
	if testing.Short() {
		t.Skip("Long run not good for Travis")
	}
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	clients := make(map[int]*skipchain.Client)
	for i := range [8]byte{} {
		clients[i] = skipchain.NewClient()
	}
	genesis, cerr := clients[0].CreateGenesis(el, 1, 1, nil, nil, nil)
	log.ErrFatal(cerr)

	wg := sync.WaitGroup{}
	for i := range [128]byte{} {
		wg.Add(1)
		go func(i int) {
			_, cerr := clients[i%8].GetUpdateChain(genesis.Roster, genesis.Hash)
			log.ErrFatal(cerr)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestClient_StoreSkipBlock(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := skipchain.NewClient()
	log.Lvl1("Creating root and control chain")
	genesis, cerr := c.CreateGenesis(el, 1, 1, nil, nil, nil)
	log.ErrFatal(cerr)
	el2 := onet.NewRoster(el.List[:nbrHosts-1])
	log.Lvl1("Proposing roster", el2)
	_, sb1, cerr := c.AddSkipBlock(genesis, el2, nil)
	log.ErrFatal(cerr)
	log.Lvl1("Proposing same roster again")
	_, _, cerr = c.AddSkipBlock(genesis, el2, nil)
	require.NotNil(t, cerr,
		"Appending two Blocks to the same last block should fail")
	log.Lvl1("Proposing following roster")
	_, sb1, cerr = c.AddSkipBlock(sb1, el2, []byte{1, 2, 3})
	log.ErrFatal(cerr)
	require.Equal(t, sb1.Data, []byte{1, 2, 3})
	sb2Prev, sb2, cerr := c.AddSkipBlock(sb1, el2, &testData{})
	log.ErrFatal(cerr)
	require.True(t, sb2Prev.Equal(sb1),
		"New previous should be previous latest")
	require.True(t, bytes.Equal(sb2Prev.ForwardLink[0].Hash, sb2.Hash),
		"second should point to third SkipBlock")

	log.Lvl1("Checking update-chain")
	var updates []*skipchain.SkipBlock
	// Check if we get a conode that doesn't know about the latest block.
	for i := 0; i < 10; i++ {
		updates, cerr = c.GetUpdateChain(genesis.Roster, genesis.Hash)
		log.ErrFatal(cerr)
	}
	if len(updates) != 4 {
		t.Fatal("Should now have four Blocks to go from Genesis to current, but have", len(updates), genesis, sb2)
	}
	if !updates[len(updates)-1].Equal(sb2) {
		t.Fatal("Last block in update-chain should be last block added")
	}
	c.Close()
}

func TestClient_GetAllSkipchains(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := skipchain.NewClient()
	log.Lvl1("Creating root and control chain")
	sb1, cerr := c.CreateGenesis(el, 1, 1, skipchain.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	_, _, cerr = c.AddSkipBlock(sb1, el, nil)
	log.ErrFatal(cerr)
	sb2, cerr := c.CreateGenesis(el, 1, 1, skipchain.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	sb1id := sb1.SkipChainID()
	sb2id := sb2.SkipChainID()

	sbs, cerr := c.GetAllSkipchains(el.List[0])
	require.Equal(t, 2, len(sbs.SkipChains))
	sbs1id := sbs.SkipChains[0].SkipChainID()
	sbs2id := sbs.SkipChains[1].SkipChainID()
	require.True(t, sb1id.Equal(sbs1id) || sb1id.Equal(sbs2id))
	require.True(t, sb1id.Equal(sbs2id) || sb2id.Equal(sbs2id))
	require.NotEmpty(t, sb1id, sb2id)
}

func TestClient_UpdateBunch(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest()
	_, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := skipchain.NewClient()
	log.Lvl1("Creating root and control chain")
	genesis, cerr := c.CreateGenesis(ro, 1, 1, skipchain.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	_, sb1, cerr := c.AddSkipBlock(genesis, ro, nil)
	log.ErrFatal(cerr)
	_, _, cerr = c.AddSkipBlock(sb1, ro, nil)
	log.ErrFatal(cerr)

	bunch := skipchain.NewSkipBlockBunch(genesis)
	cerr = c.BunchUpdate(bunch)
	log.ErrFatal(cerr)
	require.Equal(t, 2, bunch.Latest.Index)
}

type testData struct {
	A int
	B string
}

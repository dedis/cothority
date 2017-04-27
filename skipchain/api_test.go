package skipchain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"bytes"

	"sync"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	network.RegisterMessage(&testData{})
}

func TestClient_CreateGenesis(t *testing.T) {
	l := onet.NewTCPTest()
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := newTestClient(l)
	_, cerr := c.CreateGenesis(roster, 1, 1, VerificationNone,
		[]byte{1, 2, 3}, nil)
	require.NotNil(t, cerr)
	_, cerr = c.CreateGenesis(roster, 1, 0, VerificationNone,
		&testData{}, nil)
	require.NotNil(t, cerr)
	_, cerr = c.CreateGenesis(roster, 1, 1, VerificationNone,
		&testData{}, nil)
	require.Nil(t, cerr)
	_, _, cerr = c.CreateRootControl(roster, roster, nil, 1, 1, 0)
	require.NotNil(t, cerr)
}

func TestClient_CreateRootControl(t *testing.T) {
	l := onet.NewTCPTest()
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := newTestClient(l)
	_, _, cerr := c.CreateRootControl(roster, roster, nil, 0, 0, 0)
	require.NotNil(t, cerr)
}

func TestClient_GetUpdateChain(t *testing.T) {
	if testing.Short() {
		t.Skip("Long run not good for Travis")
	}
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	clients := make(map[int]*Client)
	for i := range [8]byte{} {
		clients[i] = newTestClient(l)
	}
	_, inter, cerr := clients[0].CreateRootControl(el, el, nil, 1, 1, 1)
	log.ErrFatal(cerr)

	wg := sync.WaitGroup{}
	for i := range [128]byte{} {
		wg.Add(1)
		go func(i int) {
			_, cerr := clients[i%8].GetUpdateChain(inter.Roster, inter.Hash)
			log.ErrFatal(cerr)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestClient_CreateRootInter(t *testing.T) {
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c := newTestClient(l)
	root, inter, cerr := c.CreateRootControl(el, el, nil, 1, 1, 1)
	log.ErrFatal(cerr)
	if root == nil || inter == nil {
		t.Fatal("Pointers are nil")
	}
	log.ErrFatal(root.VerifyForwardSignatures(),
		"Root signature invalid:")
	log.ErrFatal(inter.VerifyForwardSignatures(),
		"Root signature invalid:")
	update, cerr := c.GetUpdateChain(root.Roster, root.Hash)
	log.ErrFatal(cerr)
	root = update.Update[0]
	require.True(t, root.ChildSL[0].Equal(inter.Hash), "Root doesn't point to intermediate")
	if !bytes.Equal(inter.ParentBlockID, root.Hash) {
		t.Fatal("Intermediate doesn't point to root")
	}
}

func TestClient_StoreSkipBlock(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating root and control chain")
	_, inter, cerr := c.CreateRootControl(el, el, nil, 1, 1, 1)
	log.ErrFatal(cerr)
	el2 := onet.NewRoster(el.List[:nbrHosts-1])
	log.Lvl1("Proposing roster", el2)
	var sb1 *StoreSkipBlockReply
	sb1, cerr = c.StoreSkipBlock(inter, el2, nil)
	log.ErrFatal(cerr)
	log.Lvl1("Proposing same roster again")
	_, cerr = c.StoreSkipBlock(inter, el2, nil)
	require.NotNil(t, cerr,
		"Appending two Blocks to the same last block should fail")
	log.Lvl1("Proposing following roster")
	sb2, cerr := c.StoreSkipBlock(sb1.Latest, el2, []byte{1, 2, 3})
	require.NotNil(t, cerr)
	sb2, cerr = c.StoreSkipBlock(sb1.Latest, el2, &testData{})
	log.ErrFatal(cerr)
	require.True(t, sb2.Previous.Equal(sb1.Latest),
		"New previous should be previous latest")
	require.True(t, bytes.Equal(sb2.Previous.ForwardLink[0].Hash, sb2.Latest.Hash),
		"second should point to third SkipBlock")

	log.Lvl1("Checking update-chain")
	var updates *GetUpdateChainReply
	// Check if we get a conode that doesn't know about the latest block.
	for i := 0; i < 10; i++ {
		updates, cerr = c.GetUpdateChain(inter.Roster, inter.Hash)
		log.ErrFatal(cerr)
	}
	if len(updates.Update) != 3 {
		t.Fatal("Should now have three Blocks to go from Genesis to current, but have", len(updates.Update), inter, sb2)
	}
	if !updates.Update[2].Equal(sb2.Latest) {
		t.Fatal("Last block in update-chain should be last block added")
	}
	c.Close()
}

func TestClient_GetAllSkipchains(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating root and control chain")
	sb1, cerr := c.CreateGenesis(el, 1, 1, VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	_, cerr = c.StoreSkipBlock(sb1, el, nil)
	log.ErrFatal(cerr)
	sb2, cerr := c.CreateGenesis(el, 1, 1, VerificationNone, nil, nil)
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

func newTestClient(l *onet.LocalTest) *Client {
	c := NewClient()
	c.Client = l.NewClient("Skipchain")
	return c
}

type testData struct {
	A int
	B string
}

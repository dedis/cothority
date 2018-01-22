package skipchain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"bytes"

	"sync"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

func init() {
	network.RegisterMessage(&testData{})
}

func TestClient_CreateGenesis(t *testing.T) {
	l := onet.NewTCPTest(Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := newTestClient(l)
	_, cerr := c.CreateGenesis(roster, 1, 1, VerificationNone,
		[]byte{1, 2, 3}, nil)
	require.Nil(t, cerr)
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
	l := onet.NewTCPTest(Suite)
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
	l := onet.NewTCPTest(Suite)
	_, ro, _ := l.GenTree(5, true)
	defer l.CloseAll()

	clients := make(map[int]*Client)
	for i := range [8]byte{} {
		clients[i] = newTestClient(l)
	}
	_, inter, cerr := clients[0].CreateRootControl(ro, ro, nil, 1, 1, 1)
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
	l := onet.NewTCPTest(Suite)
	_, ro, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c := newTestClient(l)
	root, inter, cerr := c.CreateRootControl(ro, ro, nil, 1, 1, 1)
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
	l := onet.NewTCPTest(Suite)
	_, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating root and control chain")
	_, inter, cerr := c.CreateRootControl(ro, ro, nil, 1, 1, 1)
	log.ErrFatal(cerr)
	ro2 := onet.NewRoster(ro.List[:nbrHosts-1])
	log.Lvl1("Proposing roster", ro2)
	var sb1 *StoreSkipBlockReply
	sb1, cerr = c.StoreSkipBlock(inter, ro2, nil)
	log.ErrFatal(cerr)
	log.Lvl1("Proposing same roster again")
	_, cerr = c.StoreSkipBlock(inter, ro2, nil)
	require.NotNil(t, cerr,
		"Appending two Blocks to the same last block should fail")
	log.Lvl1("Proposing following roster")
	sb1, cerr = c.StoreSkipBlock(sb1.Latest, ro2, []byte{1, 2, 3})
	log.ErrFatal(cerr)
	require.Equal(t, sb1.Latest.Data, []byte{1, 2, 3})
	sb2, cerr := c.StoreSkipBlock(sb1.Latest, ro2, &testData{})
	log.ErrFatal(cerr)
	require.True(t, sb2.Previous.Equal(sb1.Latest),
		"New previous should be previous latest")
	require.True(t, bytes.Equal(sb2.Previous.ForwardLink[0].Hash(), sb2.Latest.Hash),
		"second should point to third SkipBlock")

	log.Lvl1("Checking update-chain")
	var updates *GetUpdateChainReply
	// Check if we get a conode that doesn't know about the latest block.
	for i := 0; i < 10; i++ {
		updates, cerr = c.GetUpdateChain(inter.Roster, inter.Hash)
		log.ErrFatal(cerr)
	}
	if len(updates.Update) != 4 {
		t.Fatal("Should now have four Blocks to go from Genesis to current, but have", len(updates.Update), inter, sb2)
	}
	if !updates.Update[len(updates.Update)-1].Equal(sb2.Latest) {
		t.Fatal("Last block in update-chain should be last block added")
	}
	c.Close()
}

func TestClient_GetAllSkipchains(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest(Suite)
	_, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating root and control chain")
	sb1, cerr := c.CreateGenesis(ro, 1, 1, VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	_, cerr = c.StoreSkipBlock(sb1, ro, nil)
	log.ErrFatal(cerr)
	sb2, cerr := c.CreateGenesis(ro, 1, 1, VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	sb1id := sb1.SkipChainID()
	sb2id := sb2.SkipChainID()

	sbs, cerr := c.GetAllSkipchains(ro.List[0])
	log.ErrFatal(cerr)
	require.Equal(t, 3, len(sbs.SkipChains))
	sbs1id := sbs.SkipChains[0].SkipChainID()
	sbs2id := sbs.SkipChains[1].SkipChainID()
	require.True(t, sb1id.Equal(sbs1id) || sb1id.Equal(sbs2id))
	require.True(t, sb1id.Equal(sbs2id) || sb2id.Equal(sbs2id))
	require.NotEmpty(t, sb1id, sb2id)
}

func TestClient_GetSingleBlockByIndex(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest(Suite)
	_, roster, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating root and control chain")
	sb1, cerr := c.CreateGenesis(roster, 1, 1, VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	reply2, cerr := c.StoreSkipBlock(sb1, roster, nil)
	log.ErrFatal(cerr)
	_, cerr = c.GetSingleBlockByIndex(roster, sb1.Hash, -1)
	require.NotNil(t, cerr)
	search, cerr := c.GetSingleBlockByIndex(roster, sb1.Hash, 0)
	log.ErrFatal(cerr)
	require.True(t, sb1.Equal(search))
	search, cerr = c.GetSingleBlockByIndex(roster, sb1.Hash, 1)
	log.ErrFatal(cerr)
	require.True(t, reply2.Latest.Equal(search))
	_, cerr = c.GetSingleBlockByIndex(roster, sb1.Hash, 2)
	require.NotNil(t, cerr)
}

func TestClient_CreateLinkPrivate(t *testing.T) {
	ls := linked(1)
	defer ls.local.CloseAll()
	require.Equal(t, 0, len(ls.service.Storage.Clients))
	cerr := ls.client.CreateLinkPrivate(ls.server.ServerIdentity, ls.servPriv, ls.pub)
	require.Nil(t, cerr)
}

func TestClient_SettingAuthentication(t *testing.T) {
	ls := linked(1)
	defer ls.local.CloseAll()
	require.Equal(t, 0, len(ls.service.Storage.Clients))
	cerr := ls.client.CreateLinkPrivate(ls.si, ls.servPriv, ls.pub)
	require.Nil(t, cerr)
	require.Equal(t, 1, len(ls.service.Storage.Clients))
}

func TestClient_Follow(t *testing.T) {
	ls := linked(3)
	defer ls.local.CloseAll()
	require.Equal(t, 0, len(ls.service.Storage.Clients))
	priv0 := ls.servPriv
	cerr := ls.client.CreateLinkPrivate(ls.si, priv0, ls.pub)
	require.Nil(t, cerr)
	priv1 := ls.local.GetPrivate(ls.servers[1])
	cerr = ls.client.CreateLinkPrivate(ls.roster.List[1], priv1, ls.servers[1].ServerIdentity.Public)
	require.Nil(t, cerr)
	priv2 := ls.local.GetPrivate(ls.servers[2])
	cerr = ls.client.CreateLinkPrivate(ls.roster.List[2], priv2, ls.servers[2].ServerIdentity.Public)
	require.Nil(t, cerr)
	log.Lvl1(ls.roster)

	// Verify that server1 doesn't allow a new skipchain using server0 and server1
	roster01 := onet.NewRoster(ls.roster.List[0:2])
	_, cerr = ls.client.CreateGenesis(roster01, 1, 1, VerificationNone, nil, nil)
	require.NotNil(t, cerr)

	roster0 := onet.NewRoster([]*network.ServerIdentity{ls.si})
	genesis, cerr := ls.client.CreateGenesisSignature(roster0, 1, 1, VerificationNone, nil, nil, priv0)
	require.Nil(t, cerr)

	// Now server1 follows skipchain from server0, so it should allow a new skipblock,
	// but not a new skipchain
	log.Lvl1("(0) Following skipchain-id only")
	cerr = ls.client.AddFollow(ls.roster.List[1], priv1, genesis.SkipChainID(),
		FollowID, NewChainStrictNodes, "")
	require.Nil(t, cerr)
	block1, cerr := ls.client.StoreSkipBlockSignature(genesis, roster01, nil, priv0)
	require.Nil(t, cerr)
	genesis1, cerr := ls.client.CreateGenesisSignature(roster01, 1, 1, VerificationNone, nil, nil, priv0)
	require.Nil(t, cerr)
	_, cerr = ls.client.StoreSkipBlockSignature(genesis1, roster01, nil, priv0)
	require.NotNil(t, cerr)

	// Now server1 follows the skipchain as a 'roster-inclusion' skipchain, so it
	// should also allow creation of a new skipchain
	log.Lvl1("(1) Following roster of skipchain")
	cerr = ls.client.AddFollow(ls.roster.List[1], priv1, genesis.SkipChainID(),
		FollowSearch, NewChainStrictNodes, "")
	require.Nil(t, cerr)
	block2, cerr := ls.client.StoreSkipBlockSignature(block1.Latest, roster01, nil, priv0)
	require.Nil(t, cerr)
	genesis2, cerr := ls.client.CreateGenesisSignature(roster01, 1, 1, VerificationNone, nil, nil, priv0)
	require.Nil(t, cerr)
	_, cerr = ls.client.StoreSkipBlockSignature(genesis2, roster01, nil, priv0)
	require.Nil(t, cerr)

	// Finally test with third server
	log.Lvl1("(1) Following skipchain-id only on server2")
	cerr = ls.client.AddFollow(ls.roster.List[2], priv2, genesis.SkipChainID(),
		FollowSearch, NewChainStrictNodes, "")
	require.NotNil(t, cerr)
	log.Lvl1("(2) Following skipchain-id only on server2")
	cerr = ls.client.AddFollow(ls.roster.List[2], priv2, genesis.SkipChainID(),
		FollowLookup, NewChainStrictNodes, ls.server.Address().NetworkAddress())
	require.Nil(t, cerr)
	_, cerr = ls.client.StoreSkipBlockSignature(block2.Latest, ls.roster, nil, priv0)
	require.Nil(t, cerr)
	_, cerr = ls.client.CreateGenesisSignature(ls.roster, 1, 1, VerificationNone, nil, nil, priv0)
	require.Nil(t, cerr)
}

func TestClient_DelFollow(t *testing.T) {
	ls := linked(3)
	defer ls.local.CloseAll()

	sb, cerr := ls.client.CreateGenesis(ls.roster, 1, 1, VerificationNone, nil, nil)
	require.Nil(t, cerr)
	cerr = ls.client.AddFollow(ls.server.ServerIdentity, ls.priv, sb.SkipChainID(),
		FollowID, NewChainNone, "")
	require.Nil(t, cerr)
	require.Equal(t, 1, len(ls.service.Storage.FollowIDs))

	cerr = ls.client.DelFollow(ls.server.ServerIdentity, ls.priv, sb.SkipChainID())
	require.Nil(t, cerr)
	require.Equal(t, 0, len(ls.service.Storage.FollowIDs))
}

func TestClient_ListFollow(t *testing.T) {
	ls := linked(3)
	defer ls.local.CloseAll()

	sb1, cerr := ls.client.CreateGenesis(ls.roster, 1, 1, VerificationNone, nil, nil)
	require.Nil(t, cerr)
	cerr = ls.client.AddFollow(ls.server.ServerIdentity, ls.priv, sb1.SkipChainID(),
		FollowID, NewChainNone, "")
	require.Nil(t, cerr)
	sb2, cerr := ls.client.CreateGenesis(ls.roster, 1, 1, VerificationNone, nil, nil)
	require.Nil(t, cerr)
	cerr = ls.client.AddFollow(ls.server.ServerIdentity, ls.priv, sb2.SkipChainID(),
		FollowLookup, NewChainNone, ls.server.ServerIdentity.Address.NetworkAddress())
	require.Nil(t, cerr)

	list, cerr := ls.client.ListFollow(ls.server.ServerIdentity, ls.priv)
	require.Nil(t, cerr)
	require.Equal(t, 1, len(*list.Follow))
	require.Equal(t, 1, len(*list.FollowIDs))
}

type linkStruct struct {
	local    *onet.LocalTest
	roster   *onet.Roster
	servers  []*onet.Server
	server   *onet.Server
	service  *Service
	si       *network.ServerIdentity
	servPriv kyber.Scalar
	priv     kyber.Scalar
	pub      kyber.Point
	client   *Client
}

func linked(nbr int) *linkStruct {
	kp := key.NewKeyPair(Suite)
	ls := &linkStruct{
		local: onet.NewTCPTest(Suite),
		priv:  kp.Private,
		pub:   kp.Public,
	}
	ls.servers, ls.roster, _ = ls.local.GenTree(nbr, true)
	ls.server = ls.servers[0]
	ls.si = ls.server.ServerIdentity
	ls.servPriv = ls.local.GetPrivate(ls.server)
	ls.service = ls.local.GetServices(ls.servers, skipchainSID)[0].(*Service)
	ls.client = newTestClient(ls.local)
	return ls
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

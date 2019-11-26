package skipchain

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	onet.RegisterNewServiceWithSuite(testServiceName, suite, newCorruptedTestService)
	network.RegisterMessage(&testData{})
}

func TestClient_CreateGenesis(t *testing.T) {
	if testing.Short() {
		t.Skip("limiting travis time")
	}
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := newTestClient(l)
	_, err := c.CreateGenesis(roster, 1, 1, VerificationNone, []byte{1, 2, 3})
	require.Nil(t, err)
	_, err = c.CreateGenesis(roster, 1, 0, VerificationNone, &testData{})
	require.NotNil(t, err)
	_, err = c.CreateGenesis(roster, 1, 1, VerificationNone, &testData{})
	require.Nil(t, err)
}

func TestClient_CreateGenesisCorrupted(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	service := servers[0].Service(testServiceName).(*corruptedService)
	c := &Client{Client: onet.NewClient(cothority.Suite, testServiceName)}

	service.StoreSkipBlockReply = &StoreSkipBlockReply{}

	_, err := c.CreateGenesis(roster, 1, 1, VerificationNone, []byte{})
	require.Error(t, err)
	require.Equal(t, "got an empty reply", err.Error())

	sb := NewSkipBlock()
	service.StoreSkipBlockReply.Latest = sb
	sb.Roster = roster.NewRosterWithRoot(servers[1].ServerIdentity)
	sb.updateHash()
	_, err = c.CreateGenesis(roster, 1, 1, VerificationNone, []byte{})
	require.Error(t, err)
	require.Equal(t, "got a different roster", err.Error())

	sb.Roster = roster
	sb.VerifierIDs = []VerifierID{VerifyBase}
	sb.updateHash()
	_, err = c.CreateGenesis(roster, 1, 1, VerificationNone, []byte{})
	require.Error(t, err)
	require.Equal(t, "got a different list of verifiers", err.Error())

	sb.VerifierIDs = []VerifierID{}
	sb.Data = []byte{1}
	sb.updateHash()
	_, err = c.CreateGenesis(roster, 1, 1, VerificationNone, []byte{})
	require.Error(t, err)
	require.Equal(t, "data field does not match", err.Error())

	sb.Data = []byte{}
	sb.MaximumHeight = 2
	sb.updateHash()
	_, err = c.CreateGenesis(roster, 1, 1, VerificationNone, []byte{})
	require.Error(t, err)
	require.Equal(t, "got a different maximum height", err.Error())

	sb.MaximumHeight = 1
	sb.BaseHeight = 2
	sb.updateHash()
	_, err = c.CreateGenesis(roster, 1, 1, VerificationNone, []byte{})
	require.Error(t, err)
	require.Equal(t, "got a different base height", err.Error())
}

func TestClient_GetUpdateChain(t *testing.T) {
	// Create a small chain and test whether we can get from one element
	// of the chain to the last element with a valid slice of SkipBlocks
	local := onet.NewTCPTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()

	conodes := 10
	if testing.Short() {
		conodes = 3
	}
	sbCount := conodes - 1
	servers, roster, gs := local.MakeSRS(cothority.Suite, conodes, skipchainSID)
	s := gs.(*Service)

	c := newTestClient(local)

	sbs := make([]*SkipBlock, sbCount)
	var err error
	sbs[0], err = makeGenesisRosterArgs(s, onet.NewRoster(roster.List[0:2]),
		nil, VerificationNone, 2, 3)
	log.ErrFatal(err)

	log.Lvl1("Initialize skipchain.")

	for i := 1; i < sbCount; i++ {
		newSB := NewSkipBlock()
		newSB.Roster = onet.NewRoster(roster.List[i : i+2])
		service := local.Services[servers[i].ServerIdentity.ID][skipchainSID].(*Service)
		log.Lvl2("Storing skipblock", i, servers[i].ServerIdentity, newSB.Roster.List)
		reply, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbs[i-1].Hash, NewBlock: newSB})
		require.Nil(t, err)
		require.NotNil(t, reply.Latest)
		sbs[i] = reply.Latest
	}

	for i := 0; i < sbCount; i++ {
		sbc, err := c.GetUpdateChain(sbs[i].Roster, sbs[i].Hash)
		require.Nil(t, err)

		require.True(t, len(sbc.Update) > 0, "Empty update-chain")
		if !sbc.Update[0].Equal(sbs[i]) {
			t.Fatal("First hash is not from our SkipBlock")
		}

		if !sbc.Update[len(sbc.Update)-1].Equal(sbs[sbCount-1]) {
			log.Lvl2(sbc.Update[len(sbc.Update)-1].Hash)
			log.Lvl2(sbs[sbCount-1].Hash)
			t.Fatal("Last Hash is not equal to last SkipBlock for", i)
		}
		for up, sb1 := range sbc.Update {
			log.ErrFatal(sb1.VerifyForwardSignatures())
			if up < len(sbc.Update)-1 {
				sb2 := sbc.Update[up+1]
				h1 := sb1.Height
				h2 := sb2.Height
				height := h1
				if h2 < height {
					height = h2
				}
				require.True(t, sb1.ForwardLink[height-1].To.Equal(sb2.Hash),
					"Forward-pointer[%v/%v] of update %v %x is different from hash in %v %x",
					height-1, len(sb1.ForwardLink), up, sb1.ForwardLink[height-1].To, up+1, sb2.Hash)
			}
		}
	}
}

func TestClient_StoreSkipBlock(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating chain")
	inter, err := c.CreateGenesis(ro, 1, 1, VerificationNone, nil)
	log.ErrFatal(err)
	ro2 := onet.NewRoster(ro.List[:nbrHosts-1])
	log.Lvl1("Proposing roster", ro2)
	var sb1 *StoreSkipBlockReply
	sb1, err = c.StoreSkipBlock(inter, ro2, nil)
	log.ErrFatal(err)
	// This now works, because in order to implement concurrent writes
	// correctly, we need to have StoreSkipBlock advance latest to the
	// true latest block, atomically.
	//log.Lvl1("Proposing same roster again")
	//_, err = c.StoreSkipBlock(inter, ro2, nil)
	//require.NotNil(t, err,
	//	"Appending two Blocks to the same last block should fail")
	log.Lvl1("Proposing following roster")
	sb1, err = c.StoreSkipBlock(sb1.Latest, ro2, []byte{1, 2, 3})
	log.ErrFatal(err)
	require.Equal(t, sb1.Latest.Data, []byte{1, 2, 3})
	sb2, err := c.StoreSkipBlock(sb1.Latest, ro2, &testData{})
	log.ErrFatal(err)
	require.True(t, sb2.Previous.Equal(sb1.Latest),
		"New previous should be previous latest")
	require.True(t, bytes.Equal(sb2.Previous.ForwardLink[0].To, sb2.Latest.Hash),
		"second should point to third SkipBlock")

	log.Lvl1("Checking update-chain")
	var updates *GetUpdateChainReply
	// Check if we get a conode that doesn't know about the latest block.
	for i := 0; i < 10; i++ {
		updates, err = c.GetUpdateChain(inter.Roster, inter.Hash)
		log.ErrFatal(err)
	}
	if len(updates.Update) != 4 {
		t.Fatal("Should now have four Blocks to go from Genesis to current, but have", len(updates.Update), inter, sb2)
	}
	if !updates.Update[len(updates.Update)-1].Equal(sb2.Latest) {
		t.Fatal("Last block in update-chain should be last block added")
	}
	c.Close()
}

func TestClient_StoreSkipBlockCorrupted(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest(cothority.Suite)
	servers, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	service := servers[0].Service(testServiceName).(*corruptedService)
	c := &Client{Client: onet.NewClient(cothority.Suite, testServiceName)}
	genesis := NewSkipBlock()
	genesis.Roster = ro

	service.StoreSkipBlockReply = &StoreSkipBlockReply{}

	_, err := c.StoreSkipBlock(genesis, ro, []byte{})
	require.Nil(t, err) // empty reply

	service.StoreSkipBlockReply.Previous = NewSkipBlock()
	_, err = c.StoreSkipBlock(genesis, ro, []byte{})
	require.NotNil(t, err)
	require.Equal(t, "Calculated hash does not match", err.Error())

	service.StoreSkipBlockReply.Previous = nil
	service.StoreSkipBlockReply.Latest = NewSkipBlock()
	_, err = c.StoreSkipBlock(genesis, ro, []byte{})
	require.NotNil(t, err)
	require.Equal(t, "Calculated hash does not match", err.Error())
}

func TestClient_GetAllSkipchains(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating chain with one extra block")
	sb1, err := c.CreateGenesis(ro, 1, 1, VerificationNone, nil)
	require.Nil(t, err)
	r, err := c.StoreSkipBlock(sb1, ro, nil)
	require.Nil(t, err)
	require.Equal(t, 1, r.Latest.Index)

	// See if it works with only one chain in the system?
	sbs, err := c.GetAllSkipchains(ro.List[0])
	require.Nil(t, err)

	// Expect 2 blocks here because GetAllSkipchains is broken and actually
	// returns all the blocks.
	// If GetAllSkipchains did what it said, we should expect 1.
	require.Equal(t, 2, len(sbs.SkipChains))
}

func TestClient_GetAllSkipChainIDs(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest(cothority.Suite)
	_, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)

	log.Lvl1("Creating chain 1 with one extra block")
	sb1, err := c.CreateGenesis(ro, 1, 1, VerificationNone, nil)
	require.Nil(t, err)
	r1, err := c.StoreSkipBlock(sb1, ro, nil)
	require.Nil(t, err)
	require.Equal(t, 1, r1.Latest.Index)

	log.Lvl1("Creating chain 2 with one extra block")
	sb2, err := c.CreateGenesis(ro, 1, 1, VerificationNone, nil)
	require.Nil(t, err)
	r2, err := c.StoreSkipBlock(sb2, ro, nil)
	require.Nil(t, err)
	require.Equal(t, 1, r2.Latest.Index)

	reply, err := c.GetAllSkipChainIDs(ro.List[0])
	require.Nil(t, err)
	require.Equal(t, 2, len(reply.IDs))

	// We don't know what order they come back out, but they both have to be there.
	if reply.IDs[0].Equal(sb1.Hash) {
		require.True(t, reply.IDs[1].Equal(sb2.Hash))
	} else {
		require.True(t, reply.IDs[1].Equal(sb1.Hash))
	}
}

func TestClient_GetSingleBlock(t *testing.T) {
	nbrHosts := 1
	l := onet.NewTCPTest(cothority.Suite)
	servers, ro, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	service := servers[0].Service(testServiceName).(*corruptedService)
	c := &Client{Client: onet.NewClient(cothority.Suite, testServiceName)}

	sb := NewSkipBlock()
	sb.Roster = ro
	sb.updateHash()
	service.SkipBlock = sb

	ret, err := c.GetSingleBlock(ro, sb.Hash)
	require.Nil(t, err)
	require.Equal(t, ret.Hash, sb.Hash)

	service.SkipBlock = NewSkipBlock()
	service.SkipBlock.Roster = ro
	_, err = c.GetSingleBlock(ro, sb.Hash)
	require.NotNil(t, err)
	require.Equal(t, "Calculated hash does not match", err.Error())

	service.SkipBlock.updateHash()
	_, err = c.GetSingleBlock(ro, SkipBlockID{})
	require.NotNil(t, err)
	require.Equal(t, "Got the wrong block in return", err.Error())
}

func TestClient_GetSingleBlockByIndex(t *testing.T) {
	nbrHosts := 3
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := newTestClient(l)
	log.Lvl1("Creating root and control chain")
	nbrBlocks := 10
	blocks := make([]*SkipBlock, nbrBlocks)
	var err error
	blocks[0], err = c.CreateGenesis(roster, 2, 4, VerificationNone, nil)
	log.ErrFatal(err)
	for i := 1; i < nbrBlocks; i++ {
		reply, err := c.StoreSkipBlock(blocks[0], roster, nil)
		log.ErrFatal(err)
		blocks[i] = reply.Latest
	}

	// hand-calculated links table
	// This should be the number of forward links for every block:
	//   4, 1, 2, 1, 3, 1, 2, 1, 4, 1, 2
	// and then do the links in your head ;)
	links := []int{1, 2, 2, 3, 2, 3, 3, 4, 2, 3}

	// 0
	sb1 := blocks[0]
	search, err := c.GetSingleBlockByIndex(roster, sb1.Hash, 0)
	log.ErrFatal(err)
	require.True(t, sb1.Equal(search.SkipBlock))
	require.Equal(t, links[0], len(search.Links))
	require.True(t, sb1.SkipChainID().Equal(search.Links[0].To))
	require.True(t, sb1.Roster.Aggregate.Equal(search.Links[0].NewRoster.Aggregate))

	// 1..nbrBlocks
	for i := 1; i < nbrBlocks; i++ {
		log.Lvl1("Creating new block", i)
		search, err = c.GetSingleBlockByIndex(roster, sb1.SkipChainID(), i)
		log.ErrFatal(err)
		require.True(t, blocks[i].Hash.Equal(search.SkipBlock.Hash))
		require.Equal(t, links[i], len(search.Links))
		for _, link := range search.Links[1:] {
			require.Nil(t, link.VerifyWithScheme(suite, sb1.Roster.ServicePublics(ServiceName), BdnSignatureSchemeIndex))
		}
	}

	// non existing
	log.Lvl1("Checking last link")
	_, err = c.GetSingleBlockByIndex(roster, sb1.Hash, nbrBlocks)
	require.Error(t, err)
}

func TestClient_GetSingleBlockByIndexCorrupted(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(1, true)
	defer l.CloseAll()

	service := servers[0].Service(testServiceName).(*corruptedService)
	c := &Client{Client: onet.NewClient(cothority.Suite, testServiceName)}

	genesis := NewSkipBlock()
	genesis.Roster = roster
	genesis.updateHash()

	service.GetSingleBlockByIndexReply = &GetSingleBlockByIndexReply{}
	_, err := c.GetSingleBlockByIndex(roster, genesis.Hash, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "got an empty reply")

	sb := NewSkipBlock()
	service.GetSingleBlockByIndexReply.SkipBlock = sb
	_, err = c.GetSingleBlockByIndex(roster, genesis.Hash, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Calculated hash does not match")

	sb.Index = 1
	sb.Roster = roster
	sb.updateHash()
	_, err = c.GetSingleBlockByIndex(roster, genesis.Hash, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "got the wrong block in reply")

	sb.Index = 0
	sb.updateHash()
	_, err = c.GetSingleBlockByIndex(roster, SkipBlockID{}, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "got a block of a different chain")
}

func TestClient_CreateLinkPrivate(t *testing.T) {
	ls := linked(1)
	defer ls.local.CloseAll()
	require.Equal(t, 0, len(ls.service.Storage.Clients))
	err := ls.client.CreateLinkPrivate(ls.server.ServerIdentity, ls.servPriv, ls.pub)
	require.Nil(t, err)
}

func TestClient_SettingAuthentication(t *testing.T) {
	ls := linked(1)
	defer ls.local.CloseAll()
	require.Equal(t, 0, len(ls.service.Storage.Clients))
	err := ls.client.CreateLinkPrivate(ls.si, ls.servPriv, ls.pub)
	require.Nil(t, err)
	require.Equal(t, 1, len(ls.service.Storage.Clients))
}

func TestClient_Follow(t *testing.T) {
	ls := linked(3)
	defer ls.local.CloseAll()
	require.Equal(t, 0, len(ls.service.Storage.Clients))
	priv0 := ls.servPriv
	err := ls.client.CreateLinkPrivate(ls.si, priv0, ls.pub)
	require.Nil(t, err)
	priv1 := ls.local.GetPrivate(ls.servers[1])
	err = ls.client.CreateLinkPrivate(ls.roster.List[1], priv1, ls.servers[1].ServerIdentity.Public)
	require.Nil(t, err)
	priv2 := ls.local.GetPrivate(ls.servers[2])
	err = ls.client.CreateLinkPrivate(ls.roster.List[2], priv2, ls.servers[2].ServerIdentity.Public)
	require.Nil(t, err)
	log.Lvl1(ls.roster)

	// Verify that server1 doesn't allow a new skipchain using server0 and server1
	roster01 := onet.NewRoster(ls.roster.List[0:2])
	_, err = ls.client.CreateGenesis(roster01, 1, 1, VerificationNone, nil)
	require.NotNil(t, err)

	roster0 := onet.NewRoster([]*network.ServerIdentity{ls.si})
	genesis, err := ls.client.CreateGenesisSignature(roster0, 1, 1, VerificationNone, nil, priv0)
	require.Nil(t, err)

	// Now server1 follows skipchain from server0, so it should allow a new skipblock,
	// but not a new skipchain
	log.Lvl1("(0) Following skipchain-id only")
	err = ls.client.AddFollow(ls.roster.List[1], priv1, genesis.SkipChainID(),
		FollowID, NewChainStrictNodes, "")
	require.Nil(t, err)
	block1, err := ls.client.StoreSkipBlockSignature(genesis, roster01, nil, priv0)
	require.Nil(t, err)
	genesis1, err := ls.client.CreateGenesisSignature(roster01, 1, 1, VerificationNone, nil, priv0)
	require.Nil(t, err)
	_, err = ls.client.StoreSkipBlockSignature(genesis1, roster01, nil, priv0)
	require.NotNil(t, err)

	// Now server1 follows the skipchain as a 'roster-inclusion' skipchain, so it
	// should also allow creation of a new skipchain
	log.Lvl1("(1) Following roster of skipchain")
	err = ls.client.AddFollow(ls.roster.List[1], priv1, genesis.SkipChainID(),
		FollowSearch, NewChainStrictNodes, "")
	require.Nil(t, err)
	block2, err := ls.client.StoreSkipBlockSignature(block1.Latest, roster01, nil, priv0)
	require.Nil(t, err)
	genesis2, err := ls.client.CreateGenesisSignature(roster01, 1, 1, VerificationNone, nil, priv0)
	require.Nil(t, err)
	_, err = ls.client.StoreSkipBlockSignature(genesis2, roster01, nil, priv0)
	require.Nil(t, err)

	// Finally test with third server
	log.Lvl1("(1) Following skipchain-id only on server2")
	err = ls.client.AddFollow(ls.roster.List[2], priv2, genesis.SkipChainID(),
		FollowSearch, NewChainStrictNodes, "")
	require.NotNil(t, err)
	log.Lvl1("(2) Following skipchain-id only on server2")
	err = ls.client.AddFollow(ls.roster.List[2], priv2, genesis.SkipChainID(),
		FollowLookup, NewChainStrictNodes, ls.server.Address().NetworkAddress())
	require.Nil(t, err)
	_, err = ls.client.StoreSkipBlockSignature(block2.Latest, ls.roster, nil, priv0)
	require.Nil(t, err)
	_, err = ls.client.CreateGenesisSignature(ls.roster, 1, 1, VerificationNone, nil, priv0)
	require.Nil(t, err)
}

func TestClient_DelFollow(t *testing.T) {
	ls := linked(3)
	defer ls.local.CloseAll()

	sb, err := ls.client.CreateGenesis(ls.roster, 1, 1, VerificationNone, nil)
	require.Nil(t, err)
	err = ls.client.AddFollow(ls.server.ServerIdentity, ls.priv, sb.SkipChainID(),
		FollowID, NewChainNone, "")
	require.Nil(t, err)
	require.Equal(t, 1, len(ls.service.Storage.FollowIDs))

	err = ls.client.DelFollow(ls.server.ServerIdentity, ls.priv, sb.SkipChainID())
	require.Nil(t, err)
	require.Equal(t, 0, len(ls.service.Storage.FollowIDs))
}

func TestClient_ListFollow(t *testing.T) {
	ls := linked(3)
	defer ls.local.CloseAll()

	sb1, err := ls.client.CreateGenesis(ls.roster, 1, 1, VerificationNone, nil)
	require.Nil(t, err)
	err = ls.client.AddFollow(ls.server.ServerIdentity, ls.priv, sb1.SkipChainID(),
		FollowID, NewChainNone, "")
	require.Nil(t, err)
	sb2, err := ls.client.CreateGenesis(ls.roster, 1, 1, VerificationNone, nil)
	require.Nil(t, err)
	err = ls.client.AddFollow(ls.server.ServerIdentity, ls.priv, sb2.SkipChainID(),
		FollowLookup, NewChainNone, ls.server.ServerIdentity.Address.NetworkAddress())
	require.Nil(t, err)

	list, err := ls.client.ListFollow(ls.server.ServerIdentity, ls.priv)
	require.Nil(t, err)
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
	kp := key.NewKeyPair(cothority.Suite)
	ls := &linkStruct{
		local: onet.NewTCPTest(cothority.Suite),
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

func TestClient_ParallelWrite(t *testing.T) {
	numClients := 10
	numWrites := 10
	if testing.Short() {
		numClients = 2
	}

	l := onet.NewTCPTest(cothority.Suite)
	svrs, ro, _ := l.GenTree(5, true)
	defer l.CloseAll()

	cl := newTestClient(l)
	msg := []byte("genesis")
	gen, err := cl.CreateGenesis(ro, 2, 10, VerificationStandard, msg)
	require.Nil(t, err)

	s := l.Services[svrs[0].ServerIdentity.ID][sid].(*Service)

	wg := sync.WaitGroup{}

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			cl := newTestClient(l)
			msg := []byte(fmt.Sprintf("hello from client %v", i))

			for j := 0; j < numWrites; j++ {
				_, err := cl.StoreSkipBlock(gen, nil, msg)
				require.Nil(t, err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	num := s.db.Length()
	// plus 1 for the genesis block
	expected := numClients*numWrites + 1
	require.Equal(t, expected, num)

	// Read the chain back, check it.
	reply, err := cl.GetUpdateChain(ro, gen.SkipChainID())
	require.Nil(t, err)
	for i, sb := range reply.Update {
		if i == 0 {
			require.True(t, sb.SkipChainID().Equal(gen.Hash))
		} else {
			fl := reply.Update[i-1].ForwardLink
			require.True(t, sb.Hash.Equal(fl[len(fl)-1].To))
		}
	}
	require.Equal(t, reply.Update[len(reply.Update)-1].Index, expected-1)
	for i, x := range reply.Update {
		// Genesis does not match the expected string. NBD.
		if i == 0 {
			continue
		}
		msg := string(x.Data)
		if !strings.HasPrefix(msg, "hello from client ") {
			t.Errorf("block %v: %v", i, string(x.Data))
		}
	}
}

const testServiceName = "TestSkipChain"

type corruptedService struct {
	*Service

	// corrupted responses
	StoreSkipBlockReply        *StoreSkipBlockReply
	SkipBlock                  *SkipBlock
	GetSingleBlockByIndexReply *GetSingleBlockByIndexReply
}

func newCorruptedTestService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		Storage:          &Storage{},
		closing:          make(chan bool),
	}
	cs := &corruptedService{Service: s}

	err := s.RegisterHandlers(cs.StoreSkipBlock, cs.GetSingleBlock, cs.GetSingleBlockByIndex)

	return cs, err
}

func (cs *corruptedService) StoreSkipBlock(req *StoreSkipBlock) (*StoreSkipBlockReply, error) {
	return cs.StoreSkipBlockReply, nil
}

func (cs *corruptedService) GetSingleBlock(req *GetSingleBlock) (*SkipBlock, error) {
	return cs.SkipBlock, nil
}

func (cs *corruptedService) GetSingleBlockByIndex(req *GetSingleBlockByIndex) (*GetSingleBlockByIndexReply, error) {
	return cs.GetSingleBlockByIndexReply, nil
}

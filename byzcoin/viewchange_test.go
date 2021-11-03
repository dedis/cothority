package byzcoin

import (
	"fmt"
	"testing"
	"time"

	"go.dedis.ch/cothority/v3/skipchain"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin/viewchange"
	"go.dedis.ch/onet/v3/log"
)

// TestService_ViewChange is an end-to-end test for view-change. We kill the
// first nFailures nodes, where the nodes at index 0 is the current leader. The
// node at index nFailures should become the new leader. Then, we try to send a
// transaction to a follower, at index nFailures+1. The new leader (at index
// nFailures) should poll for new transactions and eventually make a new block
// containing that transaction. The new transaction should be stored on all
// followers. Finally, we bring the failed nodes back up and they should
// contain the transactions that they missed.
func TestViewChange_Basic(t *testing.T) {
	testViewChange(t, 4, 1)
}

func TestViewChange_Basic2(t *testing.T) {
	if testing.Short() {
		t.Skip("too many messages - travis looses some of the packets")
	}

	testViewChange(t, 7, 2)
}

func TestViewChange_Basic3(t *testing.T) {
	if testing.Short() {
		t.Skip("too many messages - travis looses some of the packets")
	}

	// Enough nodes and failing ones to test what happens when propagation
	// fails due to offline nodes in the higher level of the tree.
	testViewChange(t, 10, 3)
}

// This test is brittle with regard to timeouts:
//   - the given interval has been used for the 'blockInterval',
//   which doesn't exist anymore, but is still used for
//     - the timeout when adding a transaction
//     - calculating the timeouts when asking for a signature
//     - the time to wait to propagate to children
// So it's using the `SetPropagationTimeout` to tweak it a bit.
func testViewChange(t *testing.T, nHosts, nFailures int) {
	t.Skip("Skipping flaky viewchange tests - cross fingers it's OK...")
	bArgs := defaultBCTArgs
	bArgs.Nodes = nHosts
	bArgs.RotationWindow = 3
	// Give some more time on Travis to do the verifications
	bArgs.PropagationInterval = 2 * bArgs.PropagationInterval
	b := newBCTRun(t, &bArgs)
	defer b.CloseAll()

	propTimeout := time.Duration(4) * b.PropagationInterval

	// The propagation interval needs to be high enough so that the
	// sub-leaders and the leaves have the time to make sure the
	// transactions are correct.
	for _, service := range b.Services {
		service.SetPropagationTimeout(propTimeout)
	}

	// Stop the first nFailures hosts then the node at index nFailures
	// should take over.
	for i := 0; i < nFailures; i++ {
		log.Lvl1("stopping node at index", i)
		b.Services[i].TestClose()
		b.Servers[i].Pause()
	}

	log.Lvl1("creating a first tx to trigger the view-change")
	txArgs := TxArgsDefault
	txArgs.Node = nFailures
	b.SpawnDummy(&txArgs)

	gucr, err := b.Services[nFailures].skService().GetUpdateChain(&skipchain.
		GetUpdateChain{LatestID: b.Genesis.SkipChainID()})
	require.NoError(t, err)

	newRoster := gucr.Update[len(gucr.Update)-1].Roster
	log.Lvl1("Verifying roster", newRoster)
	sameRoster, err := newRoster.Equal(b.Roster)
	require.NoError(t, err)
	require.False(t, sameRoster)
	require.True(t, newRoster.List[0].Equal(
		b.Services[nFailures].ServerIdentity()))

	// try to send a transaction to the node on index nFailures+1, which is
	// a follower (not the new leader)
	log.Lvl1("Sending a transaction to the node after the new leader")
	txArgs.Node = nFailures + 1
	b.SpawnDummy(&txArgs)

	// check that the leader is updated for all nodes
	// Note: check is done after a tx has been sent so that nodes catch up if the
	// propagation failed
	log.Lvl1("Verifying leader is updated everywhere")
	for _, service := range b.Services[nFailures:] {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(b.Genesis.SkipChainID())
		require.NoError(t, err)
		require.NotNil(t, leader)
		require.True(t, leader.Equal(b.Services[nFailures].ServerIdentity()), fmt.Sprintf("%v", leader))
	}

	log.Lvl1("Creating new TX")
	b.SpawnDummy(&txArgs)

	// We need to bring the failed (the first nFailures) nodes back up and
	// check that they can synchronise to the latest state.
	for i := 0; i < nFailures; i++ {
		log.Lvl1("starting node at index", i)
		b.Servers[i].Unpause()
		require.NoError(t, b.Services[i].TestRestart())
		b.Services[i].SetPropagationTimeout(propTimeout)
	}
	b.Client.WaitPropagation(4)

	log.Lvl1("Adding two new tx for the resurrected nodes to catch up")
	for tx := 0; tx < 2; tx++ {
		b.SpawnDummy(&txArgs)
	}

	log.Lvl1("Check that last block has index == 6")
	pr, err := b.Client.GetProof(b.GenesisDarc.GetBaseID())
	require.NoError(t, err)
	require.Equal(t, 6, pr.Proof.Latest.Index)
}

// Tests that a view change can happen when the leader index is out of bound
func TestViewChange_LeaderIndex(t *testing.T) {
	bArgs := defaultBCTArgs
	bArgs.PropagationInterval = time.Second
	bArgs.Nodes = 5
	bArgs.RotationWindow = defaultRotationWindow
	b := newBCTRun(t, &bArgs)
	defer b.CloseAll()

	err := b.Services[0].sendViewChangeReq(viewchange.View{LeaderIndex: -1})
	require.Error(t, err)
	require.Equal(t, "leader index must be positive", err.Error())

	view := viewchange.View{
		ID:          b.Genesis.SkipChainID(),
		Gen:         b.Genesis.SkipChainID(),
		LeaderIndex: 7,
	}
	for i := 0; i < 5; i++ {
		b.Services[i].viewChangeMan.addReq(viewchange.InitReq{
			SignerID: b.Services[i].ServerIdentity().ID,
			View:     view,
		})
		err := b.Services[i].sendViewChangeReq(view)
		require.NoError(t, err)
	}

	time.Sleep(2 * b.PropagationInterval)

	for _, service := range b.Services {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(b.Genesis.SkipChainID())
		require.NoError(t, err)
		require.NotNil(t, leader)
		require.True(t, leader.Equal(b.Services[2].ServerIdentity()))
	}
}

// Test that old states of a view change that got stuck in the middle of the protocol
// are correctly cleaned if a new block is discovered.
func TestViewChange_LostSync(t *testing.T) {
	bArgs := defaultBCTArgs
	bArgs.Nodes = 5
	bArgs.PropagationInterval = time.Second
	bArgs.RotationWindow = defaultRotationWindow
	b := newBCTRun(t, &bArgs)
	defer b.CloseAll()

	target := b.Servers[1].ServerIdentity

	// Simulate the beginning of a view change
	req := &viewchange.InitReq{
		SignerID: b.Services[0].ServerIdentity().ID,
		View: viewchange.View{
			ID:          b.Genesis.Hash,
			Gen:         b.Genesis.Hash,
			LeaderIndex: 3,
		},
		Signature: []byte{},
	}
	require.NoError(t, req.Sign(b.Services[0].ServerIdentity().GetPrivate()))

	err := b.Services[0].SendRaw(target, req)
	require.NoError(t, err)

	// worst case scenario where the conode lost connectivity
	// and the view change fails in the other hand so the failing
	// conode is still waiting for requests

	// then new blocks have been added
	tx1, err := createOneClientTxWithCounter(b.GenesisDarc.GetBaseID(), DummyContractName, b.Value, b.Signer, 1)
	require.NoError(t, err)
	_, err = b.Services[1].AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   b.Genesis.SkipChainID(),
		Transaction:   tx1,
		InclusionWait: 5,
	})
	require.NoError(t, err)

	// give enough time for the propagation to be processed
	time.Sleep(1 * time.Second)

	sb, err := b.Services[1].db().GetLatestByID(b.Genesis.Hash)
	require.NoError(t, err)
	require.NotEqual(t, sb.Hash, b.Genesis.Hash)

	// Start a new view change with a different block ID
	req = &viewchange.InitReq{
		SignerID: b.Services[0].ServerIdentity().ID,
		View: viewchange.View{
			ID:          sb.Hash,
			Gen:         b.Genesis.SkipChainID(),
			LeaderIndex: 3,
		},
	}
	require.NoError(t, req.Sign(b.Services[0].ServerIdentity().GetPrivate()))

	log.OutputToBuf()
	defer log.OutputToOs()

	err = b.Services[0].SendRaw(target, req)
	require.NoError(t, err)

	time.Sleep(1 * time.Second) // request handler is asynchronous
	require.NotContains(t, log.GetStdOut(), "a request has been ignored")
	log.OutputToOs()

	// make sure a view change can still happen later
	view := viewchange.View{
		ID:          sb.Hash,
		Gen:         b.Genesis.SkipChainID(),
		LeaderIndex: 3,
	}
	for i := 0; i < 4; i++ {
		err := b.Services[i].sendViewChangeReq(view)
		require.NoError(t, err)
	}
	for i := 0; i < 4; i++ {
		b.Services[i].viewChangeMan.addReq(viewchange.InitReq{
			SignerID: b.Services[i].ServerIdentity().ID,
			View:     view,
		})
	}

	log.Lvl1("Waiting for the new block to be propagated")
	b.Client.WaitPropagation(2)
	for _, service := range b.Services {
		// everyone should have the same leader after the genesis block is stored
		leader, err := service.getLeader(b.Genesis.SkipChainID())
		require.NoError(t, err)
		require.NotNil(t, leader)
		require.True(t, leader.Equal(b.Services[3].ServerIdentity()))
	}
}

// Test to make sure the view change triggers a proof propagation when a conode
// is sending request for old blocks, meaning it is out-of-sync and as the leader
// is offline, it will never catch up.
//  - Node0 - leader - stopped after creation of block #1
//  - Node3 - misses block #1, unpaused after creation of block #1
func TestViewChange_NeedCatchUp(t *testing.T) {
	t.Skip("Flaky test - skipping it")
	nodes := 4
	bArgs := defaultBCTArgs
	bArgs.Nodes = nodes
	bArgs.RotationWindow = 3
	b := newBCTRun(t, &bArgs)
	defer b.CloseAll()

	for _, service := range b.Services {
		service.SetPropagationTimeout(2 * b.PropagationInterval)
	}

	b.Services[nodes-1].TestClose()
	b.Servers[nodes-1].Pause()

	// Create a block that host 4 will miss
	log.Lvl1("Send block that node 4 will miss")
	b.SpawnDummy(nil)

	log.Lvl1("Kill the leader")
	// Kill the leader, and unpause the sleepy node
	b.Services[0].TestClose()
	b.Servers[0].Pause()
	log.Lvl1("Unpause latest node")
	b.Servers[nodes-1].Unpause()
	require.NoError(t, b.Services[nodes-1].TestRestart())

	// Trigger a viewchange
	log.Lvl1("Trigger the viewchange")
	txArgs := TxArgsDefault
	txArgs.Node = nodes - 1
	b.SpawnDummy(&txArgs)

	// Send the block again
	log.Lvl1("Sending block again")
	b.SpawnDummy(&txArgs)

	// Check that a view change was finally executed
	leader, err := b.Services[nodes-1].getLeader(b.Genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, leader)
	require.False(t, leader.Equal(b.Services[0].ServerIdentity()))
}

package eventlog

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

var tSuite = suites.MustFind("Ed25519")

// Use this block interval for logic tests. Stress test often use a different
// block interval.
var testBlockInterval = 500 * time.Millisecond

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestClient_Log(t *testing.T) {
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	err := c.Create()
	require.Nil(t, err)
	require.NotNil(t, c.Instance)

	waitForKey(t, leader.omni, c.ByzCoin.ID, c.Instance.Slice(), testBlockInterval)

	ids, err := c.Log(NewEvent("auth", "user alice logged out"),
		NewEvent("auth", "user bob logged out"),
		NewEvent("auth", "user bob logged back in"))
	require.Nil(t, err)
	require.Equal(t, 3, len(ids))
	require.Equal(t, 32, len(ids[2]))

	// Loop while we wait for the next block to be created.
	waitForKey(t, leader.omni, c.ByzCoin.ID, ids[2], testBlockInterval)

	// Check consistency and # of events.
	for i := 0; i < 10; i++ {
		leader.waitForBlock(c.ByzCoin.ID)
		if err = leader.checkBuckets(c.Instance, c.ByzCoin.ID, 3); err == nil {
			break
		}
	}

	// Fetch index, and check its length.
	idx := checkProof(t, leader.omni, c.Instance.Slice(), c.ByzCoin.ID)
	expected := len(c.Instance)
	require.Equal(t, len(idx), expected, fmt.Sprintf("index key content is %v, expected %v", len(idx), expected))

	// Fetch the bucket and check its length.
	bucketBuf := checkProof(t, leader.omni, idx, c.ByzCoin.ID)
	var b bucket
	require.Nil(t, protobuf.Decode(bucketBuf, &b))
	// The lead bucket's prev should point to the catch-all bucket.
	require.Equal(t, len(c.Instance), len(b.Prev))

	// Check the catch-all bucket.
	bucketBuf = checkProof(t, leader.omni, b.Prev, c.ByzCoin.ID)
	var b2 bucket
	require.Nil(t, protobuf.Decode(bucketBuf, &b2))
	require.Equal(t, int64(0), b2.Start)
	// The lead bucket's prev should be nil.
	require.Equal(t, 0, len(b2.Prev))

	// Use the client API to get the event back
	for _, key := range ids {
		_, err = c.GetEvent(key)
		require.Nil(t, err)
	}

	// Test naming, this is just a sanity check for eventlogs, the main
	// naming test is in the byzcoin package.
	spawnNamingTx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{
			{
				InstanceID: byzcoin.NewInstanceID(s.gen.GetBaseID()),
				Spawn: &byzcoin.Spawn{
					ContractID: byzcoin.ContractNamingID,
				},
				SignerCounter: c.incrementCtrs(),
			},
		},
	}
	require.NoError(t, spawnNamingTx.FillSignersAndSignWith(c.Signers...))
	_, err = c.ByzCoin.AddTransactionAndWait(spawnNamingTx, 10)
	require.NoError(t, err)

	namingTx, err := c.ByzCoin.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NamingInstanceID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractNamingID,
			Command:    "add",
			Args: byzcoin.Arguments{
				{
					Name:  "instanceID",
					Value: c.Instance.Slice(),
				},
				{
					Name:  "name",
					Value: []byte("myeventlog"),
				},
			},
		},
		SignerCounter: c.incrementCtrs(),
	})
	require.NoError(t, err)
	require.NoError(t, namingTx.FillSignersAndSignWith(c.Signers...))

	_, err = c.ByzCoin.AddTransactionAndWait(namingTx, 10)
	require.NoError(t, err)

	replyID, err := c.ByzCoin.ResolveInstanceID(c.DarcID, "myeventlog")
	require.NoError(t, err)
	require.Equal(t, replyID, c.Instance)
}

func TestClient_Log200(t *testing.T) {
	if testing.Short() {
		return
	}
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	err := c.Create()
	require.Nil(t, err)
	waitForKey(t, leader.omni, c.ByzCoin.ID, c.Instance.Slice(), time.Second)

	logCount := 100
	// Write the logs in chunks to make sure that the verification
	// can be done in time.
	for i := 0; i < 5; i++ {
		current := s.getCurrentBlock(t)

		start := i * logCount / 5
		for ct := start; ct < start+logCount/5; ct++ {
			_, err := c.Log(NewEvent("auth", fmt.Sprintf("user %v logged in", ct)))
			require.Nil(t, err)
		}

		s.waitNextBlock(t, current)
	}

	// Also, one call to log with a bunch of logs in it.
	for i := 0; i < 5; i++ {
		current := s.getCurrentBlock(t)

		evs := make([]Event, logCount/5)
		for j := range evs {
			evs[j] = NewEvent("auth", fmt.Sprintf("user %v logged in", j+i*logCount/5))
		}
		_, err = c.Log(evs...)
		require.Nil(t, err)

		s.waitNextBlock(t, current)
	}

	for i := 0; i < 10; i++ {
		// Apparently leader.waitForBlock isn't enough for jenkins, so
		// wait a bit longer.
		time.Sleep(s.req.BlockInterval)
		leader.waitForBlock(c.ByzCoin.ID)
		if err = leader.checkBuckets(c.Instance, c.ByzCoin.ID, 2*logCount); err == nil {
			break
		}
	}
	require.Nil(t, err)

	// Fetch index, and check its length.
	idx := checkProof(t, leader.omni, c.Instance.Slice(), c.ByzCoin.ID)
	expected := len(c.Instance)
	require.Equal(t, len(idx), expected, fmt.Sprintf("index key content is %v, expected %v", len(idx), expected))

	// Fetch the bucket and check its length.
	bucketID := idx
	var eventCount int
	var eventIDs [][]byte
	for i := 0; i < 10; i++ {
		if len(bucketID) == 0 && eventCount == 2*logCount {
			err = nil
			break
		}
		bucketBuf := checkProof(t, leader.omni, bucketID, c.ByzCoin.ID)
		var b bucket
		require.Nil(t, protobuf.Decode(bucketBuf, &b))
		require.NotEqual(t, bucketID, b.Prev)
		eventCount += len(b.EventRefs)
		eventIDs = append(eventIDs, b.EventRefs...)
		bucketID = b.Prev
		err = fmt.Errorf("Didn't finish in time. Got %d instead of %d events", eventCount, 2*logCount)
	}
	require.Nil(t, err)

	for _, eventID := range eventIDs {
		eventBuf := checkProof(t, leader.omni, eventID, c.ByzCoin.ID)
		var e Event
		require.Nil(t, protobuf.Decode(eventBuf, &e))
	}
	require.Nil(t, s.local.WaitDone(10*time.Second))
}

func TestClient_Search(t *testing.T) {
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	err := c.Create()
	require.Nil(t, err)
	waitForKey(t, leader.omni, c.ByzCoin.ID, c.Instance.Slice(), testBlockInterval)

	// Search before any events are logged.
	req := &SearchRequest{}
	resp, err := c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 0, len(resp.Events))
	require.False(t, resp.Truncated)

	// Put 20 events in with different timestamps and topics that we can search on.
	tm0 := time.Now().UnixNano()
	if (tm0 & 1) == 0 {
		// The tests below are unfortunately fragile and only
		// work when tm0 is even. Shrug.
		tm0--
	}

	logCount := 20
	for ct := int64(0); ct < int64(logCount); ct++ {
		topic := "a"
		if (ct & 1) == 0 {
			topic = "b"
		}
		_, err := c.Log(Event{Topic: topic, Content: fmt.Sprintf("test event at time %v", ct), When: tm0 + ct})
		require.Nil(t, err)
	}
	for i := 0; i < 10; i++ {
		leader.waitForBlock(c.ByzCoin.ID)
		if err = leader.checkBuckets(c.Instance, c.ByzCoin.ID, logCount); err == nil {
			break
		}
	}

	// Search for all.
	req = &SearchRequest{}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 20, len(resp.Events))

	// Search by time range.
	req = &SearchRequest{Instance: c.Instance, ID: c.ByzCoin.ID, From: tm0 + 3, To: tm0 + 8}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated)
	require.Equal(t, 5, len(resp.Events))

	// Search by topic, should find half of them.
	req = &SearchRequest{Instance: c.Instance, ID: c.ByzCoin.ID, Topic: "a"}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated)
	require.Equal(t, 10, len(resp.Events))

	// Search by time range and topic.
	req = &SearchRequest{Instance: c.Instance, ID: c.ByzCoin.ID, Topic: "a", From: tm0 + 3, To: tm0 + 8}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated)
	require.Equal(t, 3, len(resp.Events))

	// Cause truncation.
	sm := searchMax
	searchMax = 5
	req = &SearchRequest{Instance: c.Instance, ID: c.ByzCoin.ID}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 5, len(resp.Events))
	require.True(t, resp.Truncated)
	searchMax = sm

	// Put one more event on now.
	tm := time.Now().UnixNano()
	_, err = c.Log(Event{Topic: "none", Content: "one more", When: tm})
	require.Nil(t, err)
	leader.waitForBlock(c.ByzCoin.ID)
	leader.waitForBlock(c.ByzCoin.ID)

	// Search from the last event, expect only it, not previous ones.
	req = &SearchRequest{Instance: c.Instance, ID: c.ByzCoin.ID, From: tm}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 1, len(resp.Events))
	require.False(t, resp.Truncated)
}

func TestClient_StreamEvents(t *testing.T) {
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	events := []Event{
		NewEvent("auth", "user alice logged out"),
		NewEvent("auth", "user bob logged out"),
		NewEvent("auth", "user bob logged back in"),
	}

	// set up the listener
	done := make(chan bool)
	var ctr int
	var ctrMutex sync.Mutex
	h := func(e Event, sb []byte, err error) {
		ctrMutex.Lock()
		defer ctrMutex.Unlock()

		if ctr >= len(events)-1 {
			return
		}

		require.NoError(t, err)
		require.Equal(t, e.Topic, events[ctr].Topic)
		require.Equal(t, e.Content, events[ctr].Content)
		require.NotNil(t, sb)
		ctr++

		if ctr == len(events)-1 {
			close(done)
		}
	}
	go func() {
		require.NoError(t, c.StreamEvents(h))
	}()

	// we do the create after starting the listener because it'll add a new transaction that is not an event
	// so the stream function should filter it out
	require.Nil(t, c.Create())
	require.NotNil(t, c.Instance)
	waitForKey(t, leader.omni, c.ByzCoin.ID, c.Instance.Slice(), testBlockInterval)

	// log the events
	ids, err := c.Log(events...)
	require.NoError(t, err)

	// the events should've been streamed to us.
	select {
	case <-done:
	case <-time.After(testBlockInterval + time.Second):
		require.Fail(t, "should have got n transactions")
	}

	// check the proof
	for _, id := range ids {
		checkProof(t, leader.omni, id, c.ByzCoin.ID)
	}

	require.NoError(t, c.Close())
}

func TestClient_StreamEventsFrom(t *testing.T) {
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	require.Nil(t, c.Create())
	require.NotNil(t, c.Instance)
	waitForKey(t, leader.omni, c.ByzCoin.ID, c.Instance.Slice(), testBlockInterval)

	// instead of starting the streaming at the beginning, we start it after adding some events
	batch1 := []Event{
		NewEvent("auth", "user alice logged out"),
		NewEvent("auth", "user bob logged out"),
		NewEvent("auth", "user bob logged back in"),
	}
	batch2 := []Event{
		NewEvent("read", "user alice read X"),
		NewEvent("writ", "user bob wrote Y"),
		NewEvent("read", "user bob read Z"),
	}
	events := append(batch1, batch2...)

	// log the first batch
	ids, err := c.Log(batch1...)
	require.NoError(t, err)
	// Makes sure the block is processed on each conode.
	for _, srv := range s.services {
		waitForKey(t, srv.omni, c.ByzCoin.ID, ids[len(batch1)-1], testBlockInterval)
	}
	for _, id := range ids {
		checkProof(t, leader.omni, id, c.ByzCoin.ID)
	}

	// set up the listener, it starts listening from the genesis block so even when we start after logging the first
	// batch, it should see the events from the first batch
	done := make(chan bool)
	var ctr int
	var ctrMutex sync.Mutex
	h := func(e Event, sb []byte, err error) {
		ctrMutex.Lock()
		defer ctrMutex.Unlock()

		if ctr >= len(events)-1 {
			return
		}

		require.NoError(t, err)
		require.Equal(t, e.Topic, events[ctr].Topic)
		require.Equal(t, e.Content, events[ctr].Content)
		require.NotNil(t, sb)
		ctr++

		if ctr == len(events)-1 {
			close(done)
		}
	}

	wg := sync.WaitGroup{}
	go func() {
		wg.Add(1)
		defer wg.Done()

		require.NoError(t, c.StreamEventsFrom(h, c.ByzCoin.ID))
	}()

	// write the second batch
	ids2, err := c.Log(batch2...)
	require.NoError(t, err)

	// all the events should've been streamed to us.
	select {
	case <-done:
	case <-time.After(10 * testBlockInterval):
		require.Fail(t, "should have got n transactions")
	}

	// check the proof
	for _, id := range ids2 {
		checkProof(t, leader.omni, id, c.ByzCoin.ID)
	}

	require.NoError(t, c.Close())

	wg.Wait()
}

func checkProof(t *testing.T, omni *byzcoin.Service, key []byte, scID skipchain.SkipBlockID) []byte {
	req := &byzcoin.GetProof{
		Version: byzcoin.CurrentVersion,
		Key:     key,
		ID:      scID,
	}
	resp, err := omni.GetProof(req)
	require.Nil(t, err)

	p := resp.Proof
	require.True(t, p.InclusionProof.Match(key), "proof of exclusion of index")

	v0, _, _, err := p.Get(key)
	require.NoError(t, err)

	return v0
}

func waitForKey(t *testing.T, s *byzcoin.Service, scID skipchain.SkipBlockID, key []byte, interval time.Duration) {
	if len(key) == 0 {
		t.Fatal("key len", len(key))
	}
	var found bool
	var resp *byzcoin.GetProofResponse
	for ct := 0; ct < 10; ct++ {
		req := &byzcoin.GetProof{
			Version: byzcoin.CurrentVersion,
			Key:     key,
			ID:      scID,
		}
		var err error
		resp, err = s.GetProof(req)
		if err == nil {
			p := resp.Proof.InclusionProof
			if p.Match(key) {
				found = true
				break
			}
		} else {
			t.Log("err", err)
		}
		time.Sleep(interval)
	}
	if !found {
		require.Fail(t, "timeout")
	}
	_, _, _, err := resp.Proof.Get(key)
	require.NoError(t, err)
}

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
	id       skipchain.SkipBlockID
	owner    darc.Signer
	req      *byzcoin.CreateGenesisBlock
	gen      darc.Darc // the genesis darc
}

func (s *ser) close() {
	s.local.CloseAll()
}

func newSer(t *testing.T) (*ser, *Client) {
	s := &ser{
		local: onet.NewTCPTest(tSuite),
		owner: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(3, true)

	for _, sv := range s.local.GetServices(s.hosts, sid) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}

	var err error
	s.req, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, s.roster,
		[]string{"spawn:" + contractName, "invoke:" + contractName + "." + logCmd, "_name:" + contractName}, s.owner.Identity())
	if err != nil {
		t.Fatal(err)
	}
	s.gen = s.req.GenesisDarc
	s.req.BlockInterval = testBlockInterval
	cl := onet.NewClient(cothority.Suite, byzcoin.ServiceName)

	var resp byzcoin.CreateGenesisBlockResponse
	err = cl.SendProtobuf(s.roster.List[0], s.req, &resp)
	if err != nil {
		t.Fatal(err)
	}
	s.id = resp.Skipblock.Hash

	ol := byzcoin.NewClient(s.id, *s.roster)

	c := NewClient(ol)
	c.DarcID = s.gen.GetBaseID()
	c.Signers = []darc.Signer{s.owner}

	return s, c
}

func (s *ser) getCurrentBlock(t *testing.T) skipchain.SkipBlockID {
	reply, err := skipchain.NewClient().GetUpdateChain(s.roster, s.id)
	require.Nil(t, err)
	return reply.Update[len(reply.Update)-1].Hash
}

func (s *ser) waitNextBlock(t *testing.T, current skipchain.SkipBlockID) {
	for i := 0; i < 10; i++ {
		reply, err := skipchain.NewClient().GetUpdateChain(s.roster, s.id)
		require.Nil(t, err)
		if !current.Equal(reply.Update[len(reply.Update)-1].Hash) {
			return
		}
		time.Sleep(s.req.BlockInterval)
	}
	require.Fail(t, "waited too long for new block to appear")
}

// checkBuckets walks all the buckets for a given eventlog and returns an error
// if an event is in the wrong bucket. This function is useful to check the
// correctness of buckets.
func (s *Service) checkBuckets(inst byzcoin.InstanceID, id skipchain.SkipBlockID, ct0 int) error {
	v, err := s.omni.GetReadOnlyStateTrie(id)
	if err != nil {
		return err
	}
	el := eventLog{Instance: inst, v: v}

	id, b, err := el.getLatestBucket()
	if err != nil {
		return err
	}
	if b == nil {
		return errors.New("nil bucket")
	}

	// bEnd is normally updated from the last bucket's start. For the latest
	// bucket, bEnd is now.
	bEnd := time.Now().UnixNano()
	end := time.Unix(0, bEnd)

	ct := 0
	i := 0
	for {
		st := time.Unix(0, b.Start)

		// check each event
		for j, e := range b.EventRefs {
			ev, err := getEventByID(v, e)
			if err != nil {
				return err
			}
			when := time.Unix(0, ev.When)
			if when.Before(st) {
				return fmt.Errorf("bucket %v, event %v before start (%v<%v)", i, j, when, st)
			}
			if when.After(end) {
				return fmt.Errorf("bucket %v, event %v after end (%v>%v)", i, j, when, end)
			}
			ct++
		}

		// advance to prev bucket
		if b.isFirst() {
			break
		}
		bEnd = b.Start
		end = time.Unix(0, bEnd)
		id = b.Prev
		b, err = el.getBucketByID(id)
		if err != nil {
			return err
		}
		i++
	}
	if ct0 != 0 && ct0 != ct {
		return fmt.Errorf("expected %v, found %v events", ct0, ct)
	}
	return nil
}

// waitForBlock is for use in tests; it will sleep long enough to be sure that
// a block has been created.
func (s *Service) waitForBlock(scID skipchain.SkipBlockID) {
	dur, _, err := s.omni.LoadBlockInfo(scID)
	if err != nil {
		panic(err.Error())
	}
	time.Sleep(5 * dur)
}

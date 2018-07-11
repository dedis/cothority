package eventlog

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")

// Use this block interval for logic tests. Stress test often use a different
// block interval.
var testBlockInterval = 100 * time.Millisecond

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestClient_Log(t *testing.T) {
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	err := c.Create(s.gen.GetID())
	require.Nil(t, err)
	require.NotNil(t, c.EventlogID)

	waitForKey(t, leader.omni, c.OmniLedger.ID, c.EventlogID.Slice(), testBlockInterval)

	ids, err := c.Log(NewEvent("auth", "user alice logged out"),
		NewEvent("auth", "user bob logged out"),
		NewEvent("auth", "user bob logged back in"))
	require.Nil(t, err)
	require.Equal(t, 3, len(ids))
	require.Equal(t, 64, len(ids[2]))

	// Loop while we wait for the next block to be created.
	waitForKey(t, leader.omni, c.OmniLedger.ID, ids[2], testBlockInterval)

	// Check consistency and # of events.
	for i := 0; i < 10; i++ {
		leader.waitForBlock(c.OmniLedger.ID)
		if err = leader.checkBuckets(c.EventlogID, c.OmniLedger.ID, 3); err == nil {
			break
		}
	}

	// Fetch index, and check its length.
	idx := checkProof(t, leader.omni, c.EventlogID.Slice(), c.OmniLedger.ID)
	expected := 64
	require.Equal(t, len(idx), expected, fmt.Sprintf("index key content is %v, expected %v", len(idx), expected))

	// Fetch the bucket and check its length.
	bucketBuf := checkProof(t, leader.omni, idx, c.OmniLedger.ID)
	var b bucket
	require.Nil(t, protobuf.Decode(bucketBuf, &b))
	// The lead bucket's prev should point to the catch-all bucket.
	require.Equal(t, 64, len(b.Prev))

	// Check the catch-all bucket.
	bucketBuf = checkProof(t, leader.omni, b.Prev, c.OmniLedger.ID)
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
}

func TestClient_Log200(t *testing.T) {
	if testing.Short() {
		return
	}
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	err := c.Create(s.gen.GetID())
	require.Nil(t, err)
	waitForKey(t, leader.omni, c.OmniLedger.ID, c.EventlogID.Slice(), time.Second)

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
		leader.waitForBlock(c.OmniLedger.ID)
		if err = leader.checkBuckets(c.EventlogID, c.OmniLedger.ID, 2*logCount); err == nil {
			break
		}
	}
	require.Nil(t, err)

	// Fetch index, and check its length.
	idx := checkProof(t, leader.omni, c.EventlogID.Slice(), c.OmniLedger.ID)
	expected := 64
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
		bucketBuf := checkProof(t, leader.omni, bucketID, c.OmniLedger.ID)
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
		eventBuf := checkProof(t, leader.omni, eventID, c.OmniLedger.ID)
		var e Event
		require.Nil(t, protobuf.Decode(eventBuf, &e))
	}
	require.Nil(t, s.local.WaitDone(10*time.Second))
}

func checkProof(t *testing.T, omni *omniledger.Service, key []byte, scID skipchain.SkipBlockID) []byte {
	req := &omniledger.GetProof{
		Version: omniledger.CurrentVersion,
		Key:     key,
		ID:      scID,
	}
	resp, err := omni.GetProof(req)
	require.Nil(t, err)

	p := resp.Proof.InclusionProof
	require.True(t, p.Match(), "proof of exclusion of index")

	v, _ := p.Values()
	require.Equal(t, 2, len(v), "wrong values length")

	return v[0].([]byte)
}

func TestClient_Search(t *testing.T) {
	s, c := newSer(t)
	leader := s.services[0]
	defer s.close()

	err := c.Create(s.gen.GetID())
	require.Nil(t, err)
	waitForKey(t, leader.omni, c.OmniLedger.ID, c.EventlogID.Slice(), testBlockInterval)

	// Search before any events are logged.
	req := &SearchRequest{ID: c.OmniLedger.ID}
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
		leader.waitForBlock(c.OmniLedger.ID)
		if err = leader.checkBuckets(c.EventlogID, c.OmniLedger.ID, logCount); err == nil {
			break
		}
	}

	// Without EventLogID, we should get nothing
	req = &SearchRequest{ID: c.OmniLedger.ID}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 0, len(resp.Events))

	// Search for all.
	req = &SearchRequest{EventLogID: c.EventlogID, ID: c.OmniLedger.ID}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 20, len(resp.Events))

	// Search by time range.
	req = &SearchRequest{EventLogID: c.EventlogID, ID: c.OmniLedger.ID, From: tm0 + 3, To: tm0 + 8}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated)
	require.Equal(t, 5, len(resp.Events))

	// Search by topic, should find half of them.
	req = &SearchRequest{EventLogID: c.EventlogID, ID: c.OmniLedger.ID, Topic: "a"}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated)
	require.Equal(t, 10, len(resp.Events))

	// Search by time range and topic.
	req = &SearchRequest{EventLogID: c.EventlogID, ID: c.OmniLedger.ID, Topic: "a", From: tm0 + 3, To: tm0 + 8}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Truncated)
	require.Equal(t, 3, len(resp.Events))

	// Cause truncation.
	sm := searchMax
	searchMax = 5
	req = &SearchRequest{EventLogID: c.EventlogID, ID: c.OmniLedger.ID}
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
	leader.waitForBlock(c.OmniLedger.ID)
	leader.waitForBlock(c.OmniLedger.ID)

	// Search from the last event, expect only it, not previous ones.
	req = &SearchRequest{EventLogID: c.EventlogID, ID: c.OmniLedger.ID, From: tm}
	resp, err = c.Search(req)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 1, len(resp.Events))
	require.False(t, resp.Truncated)
}

func waitForKey(t *testing.T, s *omniledger.Service, scID skipchain.SkipBlockID, key []byte, interval time.Duration) [][]byte {
	if len(key) == 0 {
		t.Fatal("key len", len(key))
	}
	var found bool
	var resp *omniledger.GetProofResponse
	for ct := 0; ct < 10; ct++ {
		req := &omniledger.GetProof{
			Version: omniledger.CurrentVersion,
			Key:     key,
			ID:      scID,
		}
		var err error
		resp, err = s.GetProof(req)
		if err == nil {
			p := resp.Proof.InclusionProof
			if p.Match() {
				found = true
				break
			}
		} else {
			t.Log("err", err)
		}
		time.Sleep(interval)
	}
	if !found {
		t.Fatal("timeout")
	}
	_, vs, err := resp.Proof.KeyValue()
	require.NoError(t, err)
	return vs
}

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
	id       skipchain.SkipBlockID
	owner    darc.Signer
	req      *omniledger.CreateGenesisBlock
	gen      darc.Darc // the genesis darc
}

func (s *ser) close() {
	for _, x := range s.services {
		x.omni.ClosePolling()
	}
	s.local.CloseAll()
}

func newSer(t *testing.T) (*ser, *Client) {
	s := &ser{
		local: onet.NewTCPTest(tSuite),
		owner: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(2, true)

	for _, sv := range s.local.GetServices(s.hosts, sid) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}

	// And create an Omniledger to write to.
	var err error
	s.req, err = omniledger.DefaultGenesisMsg(omniledger.CurrentVersion, s.roster,
		[]string{"spawn:darc", "spawn:eventlog", "invoke:eventlog"}, s.owner.Identity())
	if err != nil {
		t.Fatal(err)
	}
	s.gen = s.req.GenesisDarc
	s.req.BlockInterval = testBlockInterval
	cl := onet.NewClient(cothority.Suite, omniledger.ServiceName)

	var resp omniledger.CreateGenesisBlockResponse
	err = cl.SendProtobuf(s.roster.List[0], s.req, &resp)
	if err != nil {
		t.Fatal(err)
	}
	s.id = resp.Skipblock.Hash

	ol := omniledger.NewClient()
	ol.Roster = s.roster
	ol.ID = s.id

	c := NewClient(ol)
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
func (s *Service) checkBuckets(objID omniledger.InstanceID, id skipchain.SkipBlockID, ct0 int) error {
	v := s.omni.GetCollectionView(id)
	el := eventLog{ID: objID.Slice(), v: v}

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

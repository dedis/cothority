package eventlog

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
)

// ServiceName is the service name for the EventLog service.
var ServiceName = "EventLog"

var sid onet.ServiceID

func init() {
	var err error
	sid, err = onet.RegisterNewService(ServiceName, newService)
	if err != nil {
		log.Fatal(err)
	}
}

// Service is the EventLog service.
type Service struct {
	*onet.ServiceProcessor
	omni         *omniledger.Service
	bucketMaxAge time.Duration
}

const defaultBlockInterval = 5 * time.Second

// waitForBlock is for use in tests; it will sleep long enough to be sure that
// a block has been created.
func (s *Service) waitForBlock(scID skipchain.SkipBlockID) {
	dur, err := s.omni.LoadBlockInterval(scID)
	if err != nil {
		panic(err.Error())
	}
	time.Sleep(5 * dur)
}

// This should be a const, but we want to be able to hack it from tests.
var searchMax = 10000

// Search will search the event log for matching entries.
func (s *Service) Search(req *SearchRequest) (*SearchResponse, error) {
	if req.ID.IsNull() {
		return nil, errors.New("skipchain ID required")
	}

	if req.To == 0 {
		req.To = time.Now().UnixNano()
	}

	v := s.omni.GetCollectionView(req.ID)
	el := &eventLog{ID: req.EventLogID.Slice(), v: v}

	id, b, err := el.getLatestBucket()
	if err == errIndexMissing {
		// There are no events yet on this chain, so return no results.
		return &SearchResponse{}, nil
	}
	if err != nil {
		return nil, err
	}
	// bEnd is normally updated from the last bucket's start. For the latest
	// bucket, bEnd is now.
	bEnd := time.Now().UnixNano()

	// Walk backwards in the bucket chain through 2 zones: first where the
	// bucket covers time that is not in our search range, and then where the buckets
	// do cover the search range. When we see a bucket that ends before our search
	// range, we can stop walking buckets.
	var buckets []*bucket
	var bids [][]byte
	for {
		if req.From > bEnd {
			// This bucket is before the search range, so we are done walking back the bucket chain.
			break
		}

		if req.To < b.Start {
			// This bucket is after the search range, so we do not add it to buckets, but
			// we keep walking up the chain.
		} else {
			buckets = append(buckets, b)
			bids = append(bids, id)
		}

		if b.isFirst() {
			break
		}
		bEnd = b.Start
		id = b.Prev
		b, err = el.getBucketByID(id)
		if err != nil {
			// This indicates that the event log data structure is wrong, so
			// we cannot claim to correctly search it. Give up instead.
			log.Errorf("expected event log bucket id %x not found: %v", b.Prev, err)
			return nil, err
		}
	}

	reply := &SearchResponse{}

	// Process the time buckets from earliest to latest so that
	// if we truncate, it is the latest events that are not returned,
	// so that they can set req.From = resp.Events[len(resp.Events)-1].When.
filter:
	for i := len(buckets) - 1; i >= 0; i-- {
		b := buckets[i]
		for _, e := range b.EventRefs {
			ev, err := getEventByID(v, e)
			if err != nil {
				log.Errorf("bucket %x points to event %x, but the event was not found: %v", bids[i], e, err)
				return nil, err
			}

			if req.From <= ev.When && ev.When < req.To {
				if req.Topic == "" || req.Topic == ev.Topic {
					reply.Events = append(reply.Events, *ev)
					if len(reply.Events) >= searchMax {
						reply.Truncated = true
						break filter
					}
				}
			}
		}
	}

	return reply, nil
}

const contractName = "eventlog"

func (s *Service) decodeAndCheckEvent(coll omniledger.CollectionView, eventBuf []byte) (*Event, error) {
	// Check the timestamp of the event: it should never be in the future,
	// and it should not be more than 5 seconds in the past. (Why 5?
	// Because a #of blocks limit is too fragile when using fast blocks for
	// tests.)
	event := &Event{}
	err := protobuf.Decode(eventBuf, event)
	if err != nil {
		return nil, err
	}
	when := time.Unix(0, event.When)
	now := time.Now()
	if when.Before(now.Add(-30 * time.Second)) {
		return nil, fmt.Errorf("event timestamp too long ago - when=%v, now=%v", when, now)
	}
	if when.After(now) {
		return nil, errors.New("event timestamp is in the future")
	}
	return event, nil
}

// invoke will add an event and update the corresponding indices.
func (s *Service) invoke(v omniledger.CollectionView, tx omniledger.Instruction, c []omniledger.Coin) ([]omniledger.StateChange, []omniledger.Coin, error) {
	// This is not strictly required, because since we know we are
	// a spawn, we know the contract comes directly from
	// tx.Spawn.ContractID.
	cid, _, err := tx.GetContractState(v)
	if err != nil {
		return nil, nil, err
	}
	if cid != contractName {
		return nil, nil, fmt.Errorf("expected contract ID to bd %s but got %s", contractName, cid)
	}

	// All the state changes, at every step, go in here.
	scs := []omniledger.StateChange{}

	eventBuf := tx.Invoke.Args.Search("event")
	if eventBuf == nil {
		return nil, nil, errors.New("expected a named argument of \"event\"")
	}

	// TODO use DeriveID
	eventKey := tx.Invoke.Args.Search("key")
	if eventKey == nil {
		return nil, nil, errors.New("expected a named argument of \"key\"")
	}
	if len(eventKey) != 64 {
		return nil, nil, fmt.Errorf("event key has an incorrect length, got %d but need 64", len(eventKey))
	}
	if !bytes.Equal(eventKey[0:32], tx.ObjectID.DarcID) {
		return nil, nil, errors.New("event key must begin with the darc ID")
	}
	eventID := omniledger.BytesToObjID(eventKey)

	event, err := s.decodeAndCheckEvent(v, eventBuf)
	if err != nil {
		return nil, nil, err
	}

	scs = append(scs, omniledger.NewStateChange(omniledger.Create, eventID, cid, eventBuf))

	// Walk from latest bucket back towards beginning looking for the right bucket.
	//
	// If you don't find a bucket with b.Start <= ev.When,
	// create a new bucket, put in the event, set the start, emit the bucket,
	// update prev in the bucket before (and also possibly the index key).
	//
	// If you find an existing latest bucket, and b.Start is more than X seconds
	// ago, make a new bucket anyway.
	//
	// If you find the right bucket, add the event and emit the updated bucket.
	// For now: buckets are allowed to grow as big as needed (but the previous
	// rule prevents buckets from getting too big by timing them out).

	el := &eventLog{ID: tx.ObjectID.Slice(), v: v}
	bID, b, err := el.getLatestBucket()
	if err != nil {
		return nil, nil, err
	}
	isHead := true

	for b != nil && !b.isFirst() {
		if b.Start <= event.When {
			break
		}
		bID = b.Prev
		b, err = el.getBucketByID(bID)
		if err != nil {
			return nil, nil, err
		}
		isHead = false
	}

	// Make a new head bucket if:
	//   No latest bucket: b == nil
	//     or
	//   Found a bucket, and it is head, and is too old
	if b == nil || isHead && time.Duration(event.When-b.Start) > s.bucketMaxAge {
		newBid := tx.DeriveID("bucket")

		if b == nil {
			// Special case: The first bucket for an eventlog
			// needs a catch-all bucket before it, in case later
			// events come in.
			catchID := tx.DeriveID("catch-all")
			newb := &bucket{
				Start:     0,
				Prev:      nil,
				EventRefs: nil,
			}
			buf, err := protobuf.Encode(newb)
			if err != nil {
				return nil, nil, err
			}
			scs = append(scs, omniledger.NewStateChange(omniledger.Create, catchID, cid, buf))
			bID = catchID.Slice()
		}

		// This new bucket will start with this event.
		newb := &bucket{
			Start:     event.When,
			Prev:      bID,
			EventRefs: [][]byte{eventKey},
		}
		buf, err := protobuf.Encode(newb)
		if err != nil {
			return nil, nil, err
		}
		scs = append(scs, omniledger.NewStateChange(omniledger.Create, newBid, cid, buf))

		// Update the pointer to the latest bucket.
		scs = append(scs, omniledger.NewStateChange(omniledger.Update, tx.ObjectID, cid, newBid.Slice()))
	} else {
		// Otherwise just add into whatever bucket we found, no matter how
		// many are already there. (Splitting buckets is hard and not important to us.)
		b.EventRefs = append(b.EventRefs, eventKey)
		bucketBuf, err := protobuf.Encode(b)
		if err != nil {
			return nil, nil, err
		}
		scs = append(scs,
			omniledger.StateChange{
				StateAction: omniledger.Update,
				ObjectID:    bID,
				ContractID:  []byte(contractName),
				Value:       bucketBuf,
			})
	}
	return scs, nil, nil
}

func (s *Service) spawn(v omniledger.CollectionView, instr omniledger.Instruction, c []omniledger.Coin) ([]omniledger.StateChange, []omniledger.Coin, error) {
	cid, _, err := instr.GetContractState(v)
	if err != nil {
		return nil, nil, err
	}
	if cid != contractName {
		return nil, nil, errors.New("invalid contract ID: " + cid)
	}

	var subID omniledger.Nonce
	copy(subID[:], instr.Hash())
	objID := omniledger.ObjectID{
		DarcID:     instr.ObjectID.DarcID,
		InstanceID: subID,
	}
	// We just need to store the key, because we make a Create state change
	// here and all the following changes are Update.
	scs := []omniledger.StateChange{omniledger.NewStateChange(omniledger.Create, objID, cid, make([]byte, 64))}
	return scs, []omniledger.Coin{}, nil
}

// contractFunction is the function that runs to process a transaction of
// type "eventlog"
func (s *Service) contractFunction(v omniledger.CollectionView, tx omniledger.Instruction, c []omniledger.Coin) ([]omniledger.StateChange, []omniledger.Coin, error) {
	if tx.GetType() == omniledger.InvokeType {
		return s.invoke(v, tx, c)
	} else if tx.GetType() == omniledger.SpawnType {
		return s.spawn(v, tx, c)
	}
	return nil, nil, errors.New("invalid action")
}

// checkBuckets walks all the buckets for a given eventlog and returns an error
// if an event is in the wrong bucket. This function is useful to check the
// correctness of buckets.
func (s *Service) checkBuckets(objID omniledger.ObjectID, id skipchain.SkipBlockID, ct0 int) error {
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

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real
// deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		omni:             c.Service(omniledger.ServiceName).(*omniledger.Service),
		// Set a relatively low time for bucketMaxAge: during peak message arrival
		// this will pretect the buckets from getting too big. During low message
		// arrival (< 1 per 5 sec) it does not create extra buckets, because time
		// periods with no events do not need buckets created for them.
		bucketMaxAge: 5 * time.Second,
	}
	if err := s.RegisterHandlers(s.Search); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}

	omniledger.RegisterContract(s, contractName, s.contractFunction)
	return s, nil
}

func getEventByID(coll omniledger.CollectionView, objID []byte) (*Event, error) {
	r, err := coll.Get(objID).Record()
	if err != nil {
		return nil, err
	}
	v, err := r.Values()
	if err != nil {
		return nil, err
	}
	newval, ok := v[0].([]byte)
	if !ok {
		return nil, errors.New("invalid value")
	}
	var e Event
	if err := protobuf.Decode(newval, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

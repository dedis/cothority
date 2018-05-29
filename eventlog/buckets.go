package eventlog

import (
	"errors"
	"time"

	"github.com/dedis/protobuf"
	"github.com/dedis/student_18_omniledger/omniledger/collection"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
)

var errIndexMissing = errors.New("index does not exist")

type bucket struct {
	Start     time.Time
	End       time.Time
	Prev      []byte
	EventRefs [][]byte
}

func (b *bucket) updateBucket(bucketObjID, eventObjID []byte, event Event) (omniledger.StateChanges, error) {
	if event.Timestamp.Before(b.Start) {
		return nil, errors.New("invalid timestamp")
	}
	if event.Timestamp.After(b.End) {
		b.End = event.Timestamp
	}
	if b.Start.Before(time.Unix(0, 0)) {
		b.Start = event.Timestamp
	}
	b.EventRefs = append(b.EventRefs, eventObjID)
	bucketBuf, err := protobuf.Encode(b)
	if err != nil {
		return nil, err
	}
	return []omniledger.StateChange{
		omniledger.StateChange{
			StateAction: omniledger.Update,
			ObjectID:    bucketObjID,
			ContractID:  []byte(contractName),
			Value:       bucketBuf,
		},
	}, nil
}

func (b *bucket) newLink(oldID, newID []byte) (omniledger.StateChanges, *bucket, error) {
	var newBucket bucket
	newBucket.Prev = oldID
	bucketBuf, err := protobuf.Encode(&newBucket)
	if err != nil {
		return nil, nil, err
	}
	return []omniledger.StateChange{
		omniledger.StateChange{
			StateAction: omniledger.Create,
			ObjectID:    newID,
			ContractID:  []byte(contractName),
			Value:       bucketBuf,
		},
	}, &newBucket, nil
}

func getLatestBucket(coll collection.Collection) ([]byte, *bucket, error) {
	bucketID, err := getIndexValue(coll)
	if err != nil {
		return nil, nil, err
	}
	if len(bucketID) != 64 {
		return nil, nil, errors.New("wrong length")
	}
	b, err := getBucketByID(coll, bucketID)
	if err != nil {
		return nil, nil, err
	}
	return bucketID, b, nil
}

func getBucketByID(coll collection.Collection, objID []byte) (*bucket, error) {
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
	var b bucket
	if err := protobuf.Decode(newval, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func getIndexValue(coll collection.Collection) ([]byte, error) {
	r, err := coll.Get(indexKey.Slice()).Record()
	if err != nil {
		return nil, err
	}
	if !r.Match() {
		return nil, errIndexMissing
	}
	v, err := r.Values()
	if err != nil {
		return nil, err
	}
	newval, ok := v[0].([]byte)
	if !ok {
		return nil, errors.New("invalid value")
	}
	return newval, nil
}

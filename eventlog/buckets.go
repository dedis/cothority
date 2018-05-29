package eventlog

import (
	"errors"
	"fmt"

	"github.com/dedis/protobuf"
	"github.com/dedis/student_18_omniledger/omniledger/collection"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
)

var errIndexMissing = errors.New("index does not exist")

type bucket struct {
	Start     int64
	End       int64
	Prev      []byte
	EventRefs [][]byte
}

func (b *bucket) updateBucket(bucketObjID, eventObjID []byte, event Event) (omniledger.StateChanges, error) {
	// TODO substitue for block interval
	if event.Timestamp+1000 < b.Start {
		return nil, fmt.Errorf("invalid timestamp %d < %d", event.Timestamp-1000, b.Start)
	}
	if b.Start == 0 {
		b.Start = event.Timestamp
	}
	if event.Timestamp > b.End {
		b.End = event.Timestamp
	}
	b.EventRefs = append(b.EventRefs, eventObjID)
	bucketBuf, err := protobuf.Encode(b)
	if err != nil {
		return nil, err
	}
	return []omniledger.StateChange{
		omniledger.StateChange{
			StateAction: omniledger.Update,
			ObjectID:    append([]byte{}, bucketObjID...),
			ContractID:  []byte(contractName),
			Value:       bucketBuf,
		},
	}, nil
}

func (b *bucket) newLink(oldID, newID, eventID []byte) (omniledger.StateChanges, *bucket, error) {
	var newBucket bucket
	newBucket.Prev = append([]byte{}, oldID...)
	newBucket.EventRefs = [][]byte{eventID}
	bucketBuf, err := protobuf.Encode(&newBucket)
	if err != nil {
		return nil, nil, err
	}
	return []omniledger.StateChange{
		omniledger.StateChange{
			StateAction: omniledger.Create,
			ObjectID:    append([]byte{}, newID...),
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

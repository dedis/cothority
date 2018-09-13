package eventlog

import (
	"bytes"
	"errors"

	omniledger "github.com/dedis/cothority/byzcoin/service"
	"github.com/dedis/protobuf"
)

var errIndexMissing = errors.New("index does not exist")

type bucket struct {
	Start     int64
	Prev      []byte
	EventRefs [][]byte
}

func (b bucket) isFirst() bool {
	return len(b.Prev) == 0
}

type eventLog struct {
	Instance omniledger.InstanceID
	v        omniledger.CollectionView
}

func (e eventLog) getLatestBucket() ([]byte, *bucket, error) {
	bucketID, err := e.getIndexValue()
	if err != nil {
		return nil, nil, err
	}
	if len(bucketID) != 32 {
		return nil, nil, errors.New("wrong length")
	}
	// The eventLog index has been initialised, but not used yet, so we
	// return an empty bucketID and an empty bucket.
	if bytes.Equal(bucketID, make([]byte, 32)) {
		return nil, nil, nil
	}
	b, err := e.getBucketByID(bucketID)
	if err != nil {
		return nil, nil, err
	}
	return bucketID, b, nil
}

func (e eventLog) getBucketByID(objID []byte) (*bucket, error) {
	r, err := e.v.Get(objID).Record()
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

func (e eventLog) getIndexValue() ([]byte, error) {
	r, err := e.v.Get(e.Instance.Slice()).Record()
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

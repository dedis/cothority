package eventlog

import (
	"bytes"
	"errors"

	"go.dedis.ch/cothority/v4/byzcoin"
	"go.dedis.ch/protobuf"
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
	Instance byzcoin.InstanceID
	v        byzcoin.ReadOnlyStateTrie
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
	v0, _, _, _, err := e.v.GetValues(objID)
	if err != nil {
		return nil, err
	}
	var b bucket
	if err := protobuf.Decode(v0, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (e eventLog) getIndexValue() ([]byte, error) {
	v0, _, _, _, err := e.v.GetValues(e.Instance.Slice())
	if err != nil {
		return nil, err
	}
	if v0 == nil {
		return nil, errIndexMissing
	}
	return v0, nil
}

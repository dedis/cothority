package service

import (
	"errors"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// DarcDB holds the database to the darcs. It has the following
// entries:
//  - DarcID - the protobuffed darc
//  - "latest" + DarcBaseID - the ID of the latest valid darc
//    attached to that BaseID
//  - "previous" + DarcID - the ID of the darc coming before
//    the given darcID
// Not added yet, but possible, would be "next" + DarcID.
type DarcDB struct {
	*bolt.DB
	bucketName []byte
}

var latestKey = []byte("latest")
var previousKey = []byte("previous")

// NewDarcDB returns an initialized DarcDB structure.
func NewDarcDB(db *bolt.DB, bn []byte) *DarcDB {
	return &DarcDB{
		DB:         db,
		bucketName: bn,
	}
}

// GetByID returns a new copy of the skip-block or nil if it doesn't exist
func (db *DarcDB) GetByID(id darc.ID) *darc.Darc {
	start := time.Now()
	defer func() {
		log.Lvl3("Time to get darc:", time.Since(start))
	}()
	var result *darc.Darc
	err := db.View(func(tx *bolt.Tx) error {
		sb, err := db.getFromTx(tx, id)
		if err != nil {
			return err
		}
		result = sb
		return nil
	})

	if err != nil {
		return nil
	}
	return result
}

// GetLatestDarc looks in the database for the latest darc
// belonging to a darc-baseID.
func (db *DarcDB) GetLatestDarc(baseID darc.ID) (d *darc.Darc, err error) {
	err = db.View(func(tx *bolt.Tx) error {
		latestKey := append(latestKey, baseID...)
		latestID := tx.Bucket(db.bucketName).Get(latestKey)
		if latestID == nil {
			return nil
		}
		d, err = db.getFromTx(tx, latestID)
		return err
	})
	return
}

// GetPathToLatest will search through the database to find
// the list of darcs that came after the one requested.
func (db *DarcDB) GetPathToLatest(start *darc.Darc) (path []*darc.Darc, err error) {
	latest, err := db.GetLatestDarc(start.GetBaseID())
	if err != nil {
		log.Error(err)
		return
	}
	if latest == nil {
		return nil, errors.New("didn't find latest")
	}
	if start.Version > latest.Version {
		return nil, errors.New("don't have a version number this big")
	}
	size := latest.Version - start.Version
	path = make([]*darc.Darc, size+1)
	path[0] = start
	if latest.Version > start.Version {
		err = db.Update(func(tx *bolt.Tx) error {

			for latest.Version > start.Version {
				path[size] = latest
				previousKey := append(previousKey, latest.GetID()...)
				val := tx.Bucket(db.bucketName).Get(previousKey)
				if val == nil {
					return errors.New("didn't find previous")
				}
				latest, err = db.getFromTx(tx, val)
				if err != nil {
					return err
				}
				size--
			}
			return nil
		})
	}
	return
}

// Store stores the given darc
func (db *DarcDB) Store(d *darc.Darc) error {
	return db.Update(func(tx *bolt.Tx) error {
		previous, err := db.GetLatestDarc(d.GetBaseID())
		if err == nil && previous != nil {
			previousKey := append(previousKey, d.GetID()...)
			if err := tx.Bucket(db.bucketName).Put(previousKey, previous.GetID()); err != nil {
				return err
			}
		}
		latestKey := append(latestKey, d.GetBaseID()...)
		if err := tx.Bucket(db.bucketName).Put(latestKey, d.GetID()); err != nil {
			return err
		}
		key := d.GetID()
		val, err := network.Marshal(d)
		if err != nil {
			return err
		}
		return tx.Bucket(db.bucketName).Put(key, val)
	})
}

// GetPrevious searches for the previous darc in the
// signature and returns it
func (db *DarcDB) GetPrevious(d *darc.Darc) (previous *darc.Darc, err error) {
	if d.Signature != nil && d.Signature.SignaturePath.Darcs != nil {
		darcs := *d.Signature.SignaturePath.Darcs
		return (darcs)[len(darcs)-1], nil
	}
	err = db.Update(func(tx *bolt.Tx) error {
		previousKey := append(previousKey, d.GetID()...)
		val := tx.Bucket(db.bucketName).Get(previousKey)
		if val == nil {
			return errors.New("didn't find previous")
		}
		previous, err = db.getFromTx(tx, val)
		return err
	})
	return
}

// getFromTx returns the darc identified by id.
// nil is returned if the key does not exist.
// An error is thrown if marshalling fails.
// The caller must ensure that this function is called from within a valid transaction.
func (db *DarcDB) getFromTx(tx *bolt.Tx, id darc.ID) (*darc.Darc, error) {
	val := tx.Bucket(db.bucketName).Get(id)
	if val == nil {
		return nil, nil
	}

	_, dMsg, err := network.Unmarshal(val, cothority.Suite)
	if err != nil {
		return nil, err
	}

	return dMsg.(*darc.Darc).Copy(), nil
}

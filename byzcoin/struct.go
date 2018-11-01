package byzcoin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/protobuf"
)

const bucketStateChangeStorage = "statechangestorage"
const defaultMaxSize = 2 * 1024 * 1024 * 1024 // 2GB
const versionLength = 64 / 8                  // bytes
const prefixLength = 32                       // bytes

var errLengthInstanceID = errors.New("InstanceID must have 32 bytes")

// StateChangeEntry is the object stored to keep track of instance history. It
// contains the state change and the block index
type StateChangeEntry struct {
	StateChange StateChange
	BlockIndex  int
	Timestamp   time.Time
}

// keyTime stores information to keep track of the age of the
// oldest version of an instance for cleaning purposes.
type keyTime struct {
	key  []byte
	time time.Time
}

type keyTimeArray []keyTime

func (kt keyTimeArray) Len() int {
	return len(kt)
}

func (kt keyTimeArray) Less(i, j int) bool {
	return kt[i].time.Before(kt[j].time)
}

func (kt keyTimeArray) Swap(i, j int) {
	kt[i], kt[j] = kt[j], kt[i]
}

// stateChangeStorage stores the state changes using their instance ID and
// their version to yeld a key. This key has the property to sort the key-value pairs
// first by instance ID and then by version so we can use the BoltDB key traversal.
// The storage cleans up by itself with respect to the parameters. For the best
// efficiency, the state changes must be added ordered by version because the
// cleaning always removes the oldest version but chooses the instance by the
// oldest state change added (it will be the same if ordered).
type stateChangeStorage struct {
	db          *bolt.DB
	size        int
	maxSize     int
	maxNbrBlock int
	sortedKeys  keyTimeArray
}

// Create a storage with a default maximum size
func newStateChangeStorage(c *onet.Context) *stateChangeStorage {
	db, _ := c.GetAdditionalBucket([]byte(bucketStateChangeStorage))
	return &stateChangeStorage{
		db:      db,
		maxSize: defaultMaxSize,
	}
}

// setMaxSize enables the cleaning of old state changes when the storage
// size is above a given threshold. Note that the value is not strict.
func (s *stateChangeStorage) setMaxSize(size int) {
	s.maxSize = size
}

// setMaxNbrBlock enables the cleaning of state changes belonging
// to blocks with an old index.
func (s *stateChangeStorage) setMaxNbrBlock(nbr int) {
	s.maxNbrBlock = nbr
}

// This will clean the oldest state changes when the total size
// is above the maximum. It will remove elements until 20% of
// the space is available.
func (s *stateChangeStorage) cleanBySize() error {
	if s.size < s.maxSize || s.maxSize == 0 {
		// nothing to clean
		return nil
	}

	if len(s.sortedKeys) == 0 {
		// This should never happen
		return errors.New("Nothing can be cleaned, something is wrong with the storage implementation")
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketStateChangeStorage))
		if err != nil {
			return err
		}

		thres := int(float64(s.maxSize) * 0.8)

		for s.size > thres {
			c := b.Cursor()
			// Get the oldest element
			key := s.sortedKeys[0].key
			k, v := c.Seek(key)
			if !bytes.Equal(key, k) {
				// This should never occur
				return errors.New("Missing key in the storage")
			}

			err := c.Delete()
			if err != nil {
				return err
			}

			s.size -= len(v)
			// Seek for the oldest element of the instance, which is
			// not necessearily the next one
			iid := key[:len(key)-versionLength]
			k, v = c.Seek(iid)

			if bytes.HasPrefix(k, iid) {
				var sce StateChangeEntry
				protobuf.Decode(v, &sce)
				s.sortedKeys[0].time = sce.Timestamp
				copy(s.sortedKeys[0].key, k)

				sort.Sort(s.sortedKeys)
			} else {
				// if none, that means it was the last
				s.sortedKeys = s.sortedKeys[1:]
			}
		}

		return nil
	})

	return err
}

// This will clean the state changes per instance where the block
// index is too low compared to the threshold
func (s *stateChangeStorage) cleanByBlock(scs StateChanges) error {
	if s.maxNbrBlock == 0 {
		return nil
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketStateChangeStorage))
		if err != nil {
			return err
		}

		done := map[string]bool{}

		// Clean only the instances where state changes have been added
		for _, sc := range scs {
			_, ok := done[string(sc.InstanceID)]
			if !ok {
				done[string(sc.InstanceID)] = true

				var sce StateChangeEntry
				blocks := map[int]bool{}
				c := b.Cursor()
				c.Seek(s.key(sc.InstanceID, math.MaxUint64))

				for k, v := c.Prev(); k != nil && bytes.HasPrefix(k, sc.InstanceID); k, v = c.Prev() {
					var err error
					if len(blocks) >= s.maxNbrBlock {
						err = c.Delete()
					} else {
						err = protobuf.Decode(v, &sce)
						blocks[sce.BlockIndex] = true
					}

					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

// this generates a storage key using the instance ID and the version
func (s *stateChangeStorage) key(iid []byte, ver uint64) []byte {
	b := bytes.Buffer{}
	b.Write(iid)
	binary.Write(&b, binary.BigEndian, &ver)

	return b.Bytes()
}

// this will clean the oldest state changes until there is enough
// space left and append the new ones
func (s *stateChangeStorage) append(scs StateChanges, sb *skipchain.SkipBlock) error {
	// Run a cleaning procedure first to insure we're not above the limit
	err := s.cleanBySize()
	if err != nil {
		return err
	}

	sortedKeys := make(keyTimeArray, len(s.sortedKeys))
	copy(sortedKeys, s.sortedKeys)

	err = s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketStateChangeStorage))
		if err != nil {
			return err
		}

		// append each list of state changes (or create the entry)
		for _, sc := range scs {
			if len(sc.InstanceID) != prefixLength {
				// as we use it as a prefix, all must have the same length
				return errLengthInstanceID
			}

			key := s.key(sc.InstanceID, sc.Version)

			now := time.Now()
			buf, err := protobuf.Encode(&StateChangeEntry{
				StateChange: sc,
				BlockIndex:  sb.Index,
				Timestamp:   now,
			})
			if err != nil {
				return err
			}

			// Check if the instance has already a value
			// and add the timestamp if not
			c := b.Cursor()
			k, _ := c.Seek(sc.InstanceID)
			if !bytes.HasPrefix(k, sc.InstanceID) {
				// no need to sort here as it's necessarily the newest
				sortedKeys = append(sortedKeys, keyTime{
					key:  key,
					time: now,
				})
			}

			err = b.Put(key, buf)
			if err != nil {
				return err
			}

			// optimization for cleaning to avoir recomputing the size
			s.size += len(buf)
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.sortedKeys = sortedKeys

	return s.cleanByBlock(scs)
}

// This will return the list of state changes for the given instance
func (s *stateChangeStorage) getAll(iid []byte) (entries []StateChangeEntry, err error) {
	if len(iid) != prefixLength {
		return nil, errLengthInstanceID
	}

	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketStateChangeStorage))

		c := b.Cursor()
		for k, v := c.Seek(iid); bytes.HasPrefix(k, iid); k, v = c.Next() {
			var sce StateChangeEntry
			err = protobuf.Decode(v, &sce)
			if err != nil {
				return err
			}

			entries = append(entries, sce)
		}

		return nil
	})

	return
}

// This will return the state change entry for the given instance and version.
// Use the bool returned value to check if the version exists
func (s *stateChangeStorage) getByVersion(iid []byte, ver uint64) (sce StateChangeEntry, ok bool, err error) {
	if len(iid) != prefixLength {
		err = errLengthInstanceID
		return
	}

	key := s.key(iid, ver)

	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketStateChangeStorage))

		v := b.Get(key)
		if v != nil {
			err := protobuf.Decode(v, &sce)
			if err != nil {
				return err
			}

			ok = true
		}

		return nil
	})

	return
}

// getLast looks for the last version of a given instance and return the entry. Use
// the bool value to know if there is a hit or not.
func (s *stateChangeStorage) getLast(iid []byte) (sce StateChangeEntry, ok bool, err error) {
	if len(iid) != prefixLength {
		err = errLengthInstanceID
		return
	}

	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketStateChangeStorage))
		c := b.Cursor()

		// Go for the next pair after the last version and go back from there
		key := new(big.Int)
		key.SetBytes(iid)
		key.Add(key, new(big.Int).SetInt64(1))

		c.Seek(key.Bytes())
		k, v := c.Prev()

		if bytes.HasPrefix(k, iid) {
			err := protobuf.Decode(v, &sce)
			if err != nil {
				return err
			}

			ok = true
		}

		return nil
	})

	return
}

// SafeAdd will add a to the value of the coin if there will be no
// overflow.
func (c *Coin) SafeAdd(a uint64) error {
	s1 := c.Value + a
	if s1 < c.Value || s1 < a {
		return errors.New("uint64 overflow")
	}
	c.Value = s1
	return nil
}

// SafeSub subtracts a from the value of the coin if there
// will be no underflow.
func (c *Coin) SafeSub(a uint64) error {
	if a <= c.Value {
		c.Value -= a
		return nil
	}
	return errors.New("uint64 underflow")
}

type bcNotifications struct {
	sync.Mutex
	// waitChannels will be informed by Service.updateTrieCallback that a
	// given ClientTransaction has been included. updateTrieCallback will
	// send true for a valid ClientTransaction and false for an invalid
	// ClientTransaction.
	waitChannels map[string]chan bool
	// blockListeners will be notified every time a block is created.
	// It is up to them to filter out block creations on chains they are not
	// interested in.
	blockListeners []chan skipchain.SkipBlockID
}

func (bc *bcNotifications) createWaitChannel(ctxHash []byte) chan bool {
	bc.Lock()
	defer bc.Unlock()
	ch := make(chan bool, 1)
	bc.waitChannels[string(ctxHash)] = ch
	return ch
}

func (bc *bcNotifications) informWaitChannel(ctxHash []byte, valid bool) {
	bc.Lock()
	defer bc.Unlock()
	ch := bc.waitChannels[string(ctxHash)]
	if ch != nil {
		ch <- valid
	}
}

func (bc *bcNotifications) deleteWaitChannel(ctxHash []byte) {
	bc.Lock()
	defer bc.Unlock()
	delete(bc.waitChannels, string(ctxHash))
}

func (bc *bcNotifications) informBlock(id skipchain.SkipBlockID) {
	bc.Lock()
	defer bc.Unlock()
	for _, x := range bc.blockListeners {
		if x != nil {
			x <- id
		}
	}
}

func (bc *bcNotifications) registerForBlocks(ch chan skipchain.SkipBlockID) int {
	bc.Lock()
	defer bc.Unlock()

	for i := 0; i < len(bc.blockListeners); i++ {
		if bc.blockListeners[i] == nil {
			bc.blockListeners[i] = ch
			return i
		}
	}

	// If we got here, no empty spots left, append and return the position of the
	// new element (on startup: after append(nil, ch), len == 1, so len-1 = index 0.
	bc.blockListeners = append(bc.blockListeners, ch)
	return len(bc.blockListeners) - 1
}

func (bc *bcNotifications) unregisterForBlocks(i int) {
	bc.Lock()
	defer bc.Unlock()
	bc.blockListeners[i] = nil
}

func (c ChainConfig) sanityCheck(old *ChainConfig) error {
	if c.BlockInterval <= 0 {
		return errors.New("block interval is less or equal to zero")
	}
	// too small would make it impossible to even send through a config update tx to fix it,
	// so don't allow that.
	if c.MaxBlockSize < 16000 {
		return errors.New("max block size is less than 16000")
	}
	// onet/network.MaxPacketSize is 10 megs, leave some headroom anyway.
	if c.MaxBlockSize > 8*1e6 {
		return errors.New("max block size is greater than 8 megs")
	}
	if len(c.Roster.List) < 3 {
		return errors.New("need at least 3 nodes to have a majority")
	}
	if old != nil {
		return old.checkNewRoster(c.Roster)
	}
	return nil
}

// checkNewRoster makes sure that the new roster follows the rules we need
// in byzcoin:
//   - no new node can join as leader
//   - only one node joining or leaving
func (c ChainConfig) checkNewRoster(newRoster onet.Roster) error {
	// Check new leader was in old roster
	if index, _ := c.Roster.Search(newRoster.List[0].ID); index < 0 {
		return errors.New("new leader must be in previous roster")
	}

	// Check we don't change more than one one
	added := 0
	oldList := onet.NewRoster(c.Roster.List)
	for _, si := range newRoster.List {
		if i, _ := oldList.Search(si.ID); i >= 0 {
			oldList.List = append(oldList.List[:i], oldList.List[i+1:]...)
		} else {
			added++
		}
	}
	if len(oldList.List)+added > 1 {
		return errors.New("can only change one node at a time - adding or removing")
	}
	return nil
}
